package query

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/odysee-api/pkg/iapi"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"
	"github.com/mitchellh/mapstructure"

	"github.com/OdyseeTeam/player-server/pkg/paid"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/ybbus/jsonrpc"
)

const (
	ClaimTagPrivate       = "c:private"
	ClaimTagUnlisted      = "c:unlisted"
	ClaimTagScheduledShow = "c:scheduled:show"
	ClaimTagScheduledHide = "c:scheduled:hide"
)

var (
	ErrClaimNotFound = errors.Err("couldn't find claim")
)

const (
	accessTypePurchase   = "purchase"
	accessTypeRental     = "rental"
	accessTypeMemberOnly = "memberonly"
	accessTypeUnlisted   = "unlisted"
	accessTypeScheduled  = "scheduled"
	accessTypeFree       = ""

	iapiTypeMembershipVod        = "Exclusive content"
	iapiTypeMembershipLiveStream = "Exclusive livestreams"

	releaseTimeRoundDownSec = 300
)

var errNeedSignedUrl = errors.Err("need signed url")
var errNeedSignedLivestreamUrl = errors.Err("need signed url")

var reAlreadyPurchased = regexp.MustCompile(`(?i)you already have a purchase`)
var rePurchaseFree = regexp.MustCompile(`(?i)does not have a purchase price`)

var timeSource TimeSource = realTimeSource{}

type TimeSource interface {
	Now() time.Time
	NowUnix() int64
	NowAfter(time.Time) bool
}

type ClaimSearchParams struct {
	ClaimID               *string  `json:"claim_id,omitempty"`
	TXID                  *string  `json:"txid,omitempty"`
	Nout                  *uint    `json:"nout,omitempty"`
	Name                  *string  `json:"name,omitempty"`
	ClaimType             []string `json:"claim_type,omitempty"`
	OrderBy               []string `json:"order_by,omitempty"`
	LimitClaimsPerChannel *int     `json:"limit_claims_per_channel,omitempty"`
	HasSource             *bool    `json:"has_source,omitempty"`
	ReleaseTime           []string `json:"release_time,omitempty"`
	ChannelIDs            []string `json:"channel_ids,omitempty"`
	AnyTags               []string `json:"any_tags,omitempty"`
	NotTags               []string `json:"not_tags,omitempty"`

	Page     uint64 `json:"page"`
	PageSize uint64 `json:"page_size"`
}

type realTimeSource struct{}

func (c *ClaimSearchParams) AnyTagsContains(tags ...string) bool {
	return sliceContains(c.AnyTags, tags...)
}

func (c *ClaimSearchParams) NotTagsContains(tags ...string) bool {
	return sliceContains(c.NotTags, tags...)
}

func (ts realTimeSource) Now() time.Time            { return time.Now() }
func (ts realTimeSource) NowUnix() int64            { return time.Now().Unix() }
func (ts realTimeSource) NowAfter(t time.Time) bool { return time.Now().After(t) }

// preflightHookGet replaces `get` request from the client with `purchase_create` + `resolve` for paid streams
// plus extra logic for looking up off-chain purchases, rentals and memberships.
// Only `streaming_url` and `purchase_receipt` (if stream has a receipt associated with it) will be returned in the response.
func preflightHookGet(caller *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
	var logger = zapadapter.NewKV(nil).With("module", "query.preprocessors")
	var (
		contentURL, metricLabel string
		isPaidStream            bool
	)
	query := GetFromContext(ctx)

	response := &jsonrpc.RPCResponse{
		ID:      query.Request.ID,
		JSONRPC: query.Request.JSONRPC,
	}
	responseResult := map[string]any{
		ParamStreamingUrl: "UNSET",
	}

	// uri vs url is not a typo, `get` query parameter will be called `uri`. It's `url(s)` in all other method calls.
	var lbryUrl string
	paramsMap := query.ParamsAsMap()
	uri, ok := paramsMap["uri"]
	if !ok {
		return nil, errors.Err("missing uri parameter for 'get' method")
	}
	lbryUrl = uri.(string)
	log := logger.With("url", lbryUrl)

	claim, err := resolve(ctx, caller, query, lbryUrl)
	if err != nil {
		return nil, err
	}
	stream := claim.Value.GetStream()
	pcfg := config.GetStreamsV6()

	hasAccess, err := checkStreamAccess(logging.AddToContext(ctx, logger), claim)
	if !hasAccess {
		return nil, err
	} else if errors.Is(err, errNeedSignedUrl) {
		src := stream.GetSource()
		if src == nil {
			m := "paid content doesn't have source data"
			log.Error(m)
			return nil, errors.Err(m)
		}
		sdHash := hex.EncodeToString(src.SdHash)
		signedUrl, err := signStreamURL77(
			pcfg["paidhost"],
			fmt.Sprintf("%s/%s/%s", pcfg["startpath"], claim.ClaimID, sdHash[:6]),
			pcfg["token"], timeSource.Now().Add(24*time.Hour).Unix())
		if err != nil {
			return nil, err
		}

		responseResult[ParamStreamingUrl] = signedUrl
		response.Result = responseResult
		return response, nil
	} else if errors.Is(err, errNeedSignedLivestreamUrl) {
		baseUrl, ok := paramsMap["base_streaming_url"].(string)
		if !ok {
			return nil, errors.Err("invalid base_streaming_url supplied")
		}
		u, err := url.Parse(baseUrl)
		if err != nil {
			return nil, errors.Err("invalid base_streaming_url supplied")
		}
		signedUrl, err := signStreamURL77(
			u.Host,
			u.Path,
			pcfg["token"], timeSource.Now().Add(24*time.Hour).Unix())
		if err != nil {
			return nil, err
		}

		responseResult[ParamStreamingUrl] = signedUrl
		response.Result = responseResult
		return response, nil
	}

	// Lbrynet paid content logic below
	var purchaseTxId string
	feeAmount := stream.GetFee().GetAmount()
	if feeAmount > 0 {
		isPaidStream = true

		if !claim.IsMyOutput {
			purchaseQuery, err := NewQuery(jsonrpc.NewRequest(
				MethodPurchaseCreate,
				map[string]any{
					"url":      lbryUrl,
					"blocking": true,
				},
			), query.WalletID)
			if err != nil {
				return nil, err
			}
			purchaseRes, err := caller.SendQuery(AttachToContext(ctx, purchaseQuery), purchaseQuery)
			if err != nil {
				return nil, err
			}
			if purchaseRes.Error != nil {
				if reAlreadyPurchased.MatchString(purchaseRes.Error.Message) {
					if claim.PurchaseReceipt == nil {
						log.Error("couldn't find purchase receipt for paid stream")
						return nil, errors.Err("couldn't find purchase receipt for paid stream")
					}
					log.Debug("purchase_create says stream is already purchased")
					purchaseTxId = claim.PurchaseReceipt.Txid
				} else if rePurchaseFree.MatchString(purchaseRes.Error.Message) {
					log.Debug("purchase_create says stream is free")
					isPaidStream = false
				} else {
					log.Warn("purchase_create errored", "err", purchaseRes.Error.Message)
					return nil, fmt.Errorf("purchase error: %v", purchaseRes.Error.Message)
				}
			} else {
				metrics.LbrytvPurchases.Inc()
				metrics.LbrytvPurchaseAmounts.Observe(float64(feeAmount))
				logger.With("made a purchase for %d LBC", feeAmount)
				// This is needed so changes can propagate for the subsequent resolve
				time.Sleep(1 * time.Second)
				claim, err = resolve(ctx, caller, query, lbryUrl)
				if err != nil {
					return nil, err
				}
				if claim.PurchaseReceipt == nil {
					log.Error("stream was paid for but receipt not found in the resolve response")
					return nil, errors.Err("couldn't find purchase receipt for paid stream")
				}
				purchaseTxId = claim.PurchaseReceipt.Txid
			}
		} else {
			purchaseTxId = "owner"
		}
	}

	if isPaidStream {
		metricLabel = metrics.LabelValuePaid
	} else {
		metricLabel = metrics.LabelValueFree
	}
	metrics.LbrytvStreamRequests.WithLabelValues(metricLabel).Inc()

	src := stream.GetSource()
	if src == nil {
		m := "stream doesn't have source data"
		log.Info(m)
		return nil, errors.Err(m)
	}
	sdHash := hex.EncodeToString(src.SdHash)[:6]
	if isPaidStream {
		size := src.GetSize()

		token, err := paid.CreateToken(claim.Name+"/"+claim.ClaimID, purchaseTxId, size, paid.ExpTenSecPer100MB)
		if err != nil {
			return nil, err
		}
		logger.Debug("stream token created", "stream", claim.Name+"/"+claim.ClaimID, "txid", purchaseTxId, "size", size)
		cdnUrl := config.Config.Viper.GetString("PaidContentURL")
		hasValidChannel := claim.SigningChannel != nil && claim.SigningChannel.ClaimID != ""
		if hasValidChannel && controversialChannels[claim.SigningChannel.ClaimID] {
			cdnUrl = strings.Replace(cdnUrl, "player.", "source.", -1)
		}
		contentURL = fmt.Sprintf(
			"%v%s/%s/%s/%s",
			cdnUrl, claim.Name, claim.ClaimID, sdHash, token)
		responseResult[ParamPurchaseReceipt] = claim.PurchaseReceipt
	} else {
		cdnUrl := config.Config.Viper.GetString("FreeContentURL")
		hasValidChannel := claim.SigningChannel != nil && claim.SigningChannel.ClaimID != ""
		if hasValidChannel && controversialChannels[claim.SigningChannel.ClaimID] {
			cdnUrl = strings.Replace(cdnUrl, "player.", "source.", -1)
		}
		contentURL = fmt.Sprintf(
			"%v%s/%s/%s",
			cdnUrl, claim.Name, claim.ClaimID, sdHash)
	}

	responseResult[ParamStreamingUrl] = contentURL

	response.Result = responseResult
	return response, nil
}

func checkStreamAccess(ctx context.Context, claim *ljsonrpc.Claim) (bool, error) {
	var (
		accessType, environ string
	)

	params := GetFromContext(ctx).ParamsAsMap()
	_, isLivestream := params["base_streaming_url"]
	if p, ok := params[iapi.ParamEnviron]; ok {
		environ, _ = p.(string)
	}

TagLoop:
	for _, t := range claim.Value.Tags {
		switch {
		case (t == "c:scheduled:hide" || t == "c:scheduled:show") && claim.Value.GetStream().ReleaseTime > timeSource.NowUnix():
			accessType = accessTypeScheduled
			break TagLoop
		case strings.HasPrefix(t, "purchase:") || t == "c:purchase":
			accessType = accessTypePurchase
			break TagLoop
		case strings.HasPrefix(t, "rental:") || t == "c:rental":
			accessType = accessTypeRental
			break TagLoop
		case t == "c:members-only":
			accessType = accessTypeMemberOnly
			break TagLoop
		case t == ClaimTagUnlisted:
			accessType = accessTypeUnlisted
			break TagLoop
		}
	}

	if accessType == accessTypeFree {
		return true, nil
	}
	if claim.IsMyOutput {
		if isLivestream {
			return true, errNeedSignedLivestreamUrl
		}
		return true, errNeedSignedUrl
	}

	signErr := errNeedSignedUrl
	if isLivestream {
		signErr = errNeedSignedLivestreamUrl
	}

	if isUserAMod(ctx, environ) {
		return true, signErr
	}

	if accessType == accessTypeUnlisted {
		// check signature and signature_ts params, error if not present
		signature, ok := params["signature"]
		if !ok {
			return false, errors.Err("missing required signature param")
		}

		signatureTS, ok := params["signature_ts"]
		if !ok {
			return false, errors.Err("missing required signature_ts param")
		}
		validateErr := ValidateSignatureFromClaim(claim, signature.(string), signatureTS.(string), claim.ClaimID)
		if validateErr != nil {
			return false, validateErr
		}
		return true, signErr
	} else if accessType == accessTypeScheduled {
		return false, errors.Err("claim release time is in the future, not ready to be viewed yet")
	}

	cu, err := auth.GetCurrentUserData(ctx)
	if err != nil {
		return false, errors.Err("no user data in context: %w", err)
	}

	iac := cu.IAPIClient()
	if iac == nil {
		return false, errors.Err("authentication required")
	}
	if environ == iapi.EnvironTest {
		iac = iac.Clone(iapi.WithEnvironment(iapi.EnvironTest))
	}

	switch accessType {
	case accessTypePurchase, accessTypeRental:
		resp := &iapi.CustomerListResponse{}
		err = iac.Call(ctx, "customer/list", map[string]string{"claim_id_filter": claim.ClaimID}, resp)
		if err != nil {
			return false, err
		}
		if len(resp.Data) == 0 {
			return false, errors.Err("no access to paid content")
		}
		purchase := resp.Data[0]
		if purchase.Status != "confirmed" {
			return false, errors.Err("unconfirmed purchase")
		}
		if accessType == accessTypeRental {
			if purchase.Type != "rental" {
				return false, errors.Err("incorrect purchase type")
			}
			if purchase.Type == "rental" && timeSource.NowAfter(purchase.ValidThrough) {
				return false, errors.Err("rental expired")
			}
		}
		return true, signErr
	case accessTypeMemberOnly:
		resp := &iapi.MembershipPerkCheck{}
		perkType := iapiTypeMembershipVod
		if isLivestream {
			perkType = iapiTypeMembershipLiveStream
		}
		err = iac.Call(ctx, "membership_perk/check", map[string]string{"claim_id": claim.ClaimID, "type": perkType}, resp)
		if err != nil {
			return false, err
		}
		if !resp.Data.HasAccess {
			return false, errors.Err("no access to members-only content")
		}
		return true, signErr
	default:
		return false, errors.Err("unknown access type")
	}
}

func resolve(ctx context.Context, c *Caller, q *Query, url string) (*ljsonrpc.Claim, error) {
	resolveQuery, err := NewQuery(jsonrpc.NewRequest(
		MethodResolve,
		map[string]any{
			"urls":                     url,
			"include_purchase_receipt": true,
			"include_protobuf":         true,
			"include_is_my_output":     true,
		},
	), q.WalletID)
	if err != nil {
		return nil, err
	}

	rawResolveResponse, err := c.SendQuery(ctx, resolveQuery)
	if err != nil {
		return nil, err
	}

	var resolveResponse ljsonrpc.ResolveResponse
	err = ljsonrpc.Decode(rawResolveResponse.Result, &resolveResponse)
	if err != nil {
		return nil, err
	}

	claim, ok := resolveResponse[url]
	if !ok {
		return nil, errors.Err("could not find a corresponding entry in the resolve response")
	}
	// Empty claim ID means that resolve error has been returned
	if claim.ClaimID == "" {
		return nil, ErrClaimNotFound
	}
	return &claim, err
}

// preflightHookClaimSearch patches tag parameters of RPC request to support scheduled and unlisted content.
func preflightHookClaimSearch(_ *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
	query := GetFromContext(ctx)
	origParams := query.ParamsAsMap()
	params := &ClaimSearchParams{}
	err := decode(origParams, params)
	if err != nil {
		return nil, fmt.Errorf("cannot decode query params: %w", err)
	}
	if params.HasSource != nil && *params.HasSource {
		if !params.AnyTagsContains(ClaimTagPrivate, ClaimTagUnlisted) {
			if !params.NotTagsContains(ClaimTagPrivate) {
				params.NotTags = append(params.NotTags, ClaimTagPrivate)
			}
			if !params.NotTagsContains(ClaimTagUnlisted) {
				params.NotTags = append(params.NotTags, ClaimTagUnlisted)
			}
			origParams["not_tags"] = params.NotTags
		}
		if !params.AnyTagsContains(ClaimTagScheduledShow, ClaimTagScheduledHide) {
			t := roundUp(timeSource.NowUnix(), releaseTimeRoundDownSec)
			if len(params.ReleaseTime) > 0 {
				params.ReleaseTime = append(params.ReleaseTime, fmt.Sprintf("<%d", t))
			} else {
				params.ReleaseTime = []string{fmt.Sprintf("<%d", t)}
			}
			origParams["release_time"] = params.ReleaseTime
		}
	}
	return nil, nil
}

func sliceContains[V comparable](cont []V, items ...V) bool {
	for _, t := range cont {
		for _, i := range items {
			if t == i {
				return true
			}
		}
	}
	return false
}

func roundUp(n, s int64) int64 {
	r := n % s
	if r == 0 {
		return n
	}
	return n + s - r
}

func decode(source, target any) error {
	config := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           target,
		TagName:          "json",
		WeaklyTypedInput: true,
		// DecodeHook: fixDecodeProto,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	err = decoder.Decode(source)
	if err != nil {
		return err
	}
	return nil
}

func getStatusResponse(_ *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
	var response map[string]any

	rawResponse := `
	{
		"blob_manager": {
		  "connections": {
			"incoming_bps": {},
			"outgoing_bps": {},
			"time": 0.0,
			"total_incoming_mbs": 0.0,
			"total_outgoing_mbs": 0.0
		  },
		  "finished_blobs": 0
		},
		"connection_status": {
		  "code": "connected",
		  "message": "No connection problems detected"
		},
		"installation_id": "692EAWhtoqDuAfQ6KHMXxFxt8tkhmt7sfprEMHWKjy5hf6PwZcHDV542VHqRnFnTCD",
		"is_running": true,
		"skipped_components": [
		  "hash_announcer",
		  "blob_server",
		  "dht"
		],
		"startup_status": {
		  "blob_manager": true,
		  "blockchain_headers": true,
		  "database": true,
		  "exchange_rate_manager": true,
		  "peer_protocol_server": true,
		  "stream_manager": true,
		  "upnp": true,
		  "wallet": true
		},
		"stream_manager": {
		  "managed_files": 1
		},
		"upnp": {
		  "aioupnp_version": "0.0.13",
		  "dht_redirect_set": false,
		  "external_ip": "127.0.0.1",
		  "gateway": "No gateway found",
		  "peer_redirect_set": false,
		  "redirects": {}
		},
		"wallet": {
		  "best_blockhash": "3d77791b9d87609a004b398e638bcdc91650247ee4448a2b30bf8474668d0ad3",
		  "blocks": 0,
		  "blocks_behind": 0,
		  "is_encrypted": false,
		  "is_locked": false
		}
	  }
	`
	json.Unmarshal([]byte(rawResponse), &response)
	rpcResponse := GetFromContext(ctx).newResponse()
	rpcResponse.Result = response
	return rpcResponse, nil
}

// isUserAMod checks and weakly returns if the user is a mod, if errors occur false is assumed
func isUserAMod(ctx context.Context, environ string) bool {
	cu, err := auth.GetCurrentUserData(ctx)
	if err != nil {
		return false
	}

	iac := cu.IAPIClient()
	if iac == nil {
		return false
	}
	if environ == iapi.EnvironTest {
		iac = iac.Clone(iapi.WithEnvironment(iapi.EnvironTest))
	}

	var userResp iapi.UserMeResponse
	err = iac.Call(ctx, "user/me", nil, &userResp)
	if err != nil {
		return false
	}
	return userResp.Success && userResp.Data.GlobalMod
}
