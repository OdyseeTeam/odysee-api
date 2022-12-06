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
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/pkg/iapi"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"

	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/player-server/pkg/paid"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/ybbus/jsonrpc"
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
)

var errNeedSignedUrl = errors.Err("need signed url")
var errNeedSignedLivestreamUrl = errors.Err("need signed url")

var reAlreadyPurchased = regexp.MustCompile(`(?i)you already have a purchase`)
var rePurchaseFree = regexp.MustCompile(`(?i)does not have a purchase price`)

// preflightHookGet replaces `get` request from the client with `purchase_create` + `resolve` for paid streams
// plus extra logic for looking up off-chain purchases, rentals and memberships.
// Only `streaming_url` and `purchase_receipt` (if stream has a receipt associated with it) will be returned in the response.
func preflightHookGet(caller *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
	var logger = zapadapter.NewKV(nil).With("module", "query.preprocessors")
	var (
		contentURL, metricLabel string
		isPaidStream            bool
	)
	query := GetQuery(ctx)

	response := &jsonrpc.RPCResponse{
		ID:      query.Request.ID,
		JSONRPC: query.Request.JSONRPC,
	}
	responseResult := map[string]interface{}{
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
	pcfg := config.GetStreamsV5()

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
		startUrl := fmt.Sprintf("%s/%s/%s", pcfg["startpath"], claim.ClaimID, sdHash[:6])
		hlsUrl := fmt.Sprintf("%s/%s/%s/master.m3u8", pcfg["hlspath"], claim.ClaimID, sdHash)
		cu, err := auth.GetCurrentUserData(ctx)
		if err != nil {
			return nil, err
		}
		ip := cu.IP()
		hlsHash := signStreamURL(hlsUrl, fmt.Sprintf("ip=%s&pass=%s", ip, pcfg["paidpass"]))

		startQuery := fmt.Sprintf("hash-hls=%s&ip=%s&pass=%s", hlsHash, ip, pcfg["paidpass"])
		responseResult[ParamStreamingUrl] = fmt.Sprintf(
			"%s%s?hash-hls=%s&ip=%s&hash=%s",
			pcfg["paidhost"], startUrl, hlsHash, ip, signStreamURL(startUrl, startQuery))
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
		cu, err := auth.GetCurrentUserData(ctx)
		if err != nil {
			return nil, err
		}
		ip := cu.IP()
		query := fmt.Sprintf("ip=%s&pass=%s", ip, pcfg["paidpass"])
		responseResult[ParamStreamingUrl] = fmt.Sprintf(
			"%s?ip=%s&hash=%s",
			baseUrl, ip, signStreamURL(u.Path, query))
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
				map[string]interface{}{
					"url":      lbryUrl,
					"blocking": true,
				},
			), query.WalletID)
			if err != nil {
				return nil, err
			}
			purchaseRes, err := caller.SendQuery(WithQuery(ctx, purchaseQuery), purchaseQuery)
			if err != nil {
				return nil, err
			}
			if purchaseRes.Error != nil {
				if reAlreadyPurchased.MatchString(purchaseRes.Error.Message) {
					log.Debug("purchase_create says stream is already purchased")
				} else if rePurchaseFree.MatchString(purchaseRes.Error.Message) {
					log.Debug("purchase_create says stream is free")
				} else {
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
		log.Error(m)
		return nil, errors.Err(m)
	}
	sdHash := hex.EncodeToString(src.SdHash)[:6]
	if isPaidStream {
		size := src.GetSize()
		fmt.Println("YOZ", claim.Name+"/"+claim.ClaimID, purchaseTxId, size, expirationFunc(size))
		token, err := paid.CreateToken(claim.Name+"/"+claim.ClaimID, purchaseTxId, size, expirationFunc)
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

	params := GetQuery(ctx).ParamsAsMap()
	_, isLivestream := params["base_streaming_url"]
	if p, ok := params[iapi.ParamEnviron]; ok {
		environ, _ = p.(string)
	}

TagLoop:
	for _, t := range claim.Value.Tags {
		switch {
		case strings.HasPrefix(t, "purchase:") || t == "c:purchase":
			accessType = accessTypePurchase
			break TagLoop
		case strings.HasPrefix(t, "rental:") || t == "c:rental":
			accessType = accessTypeRental
			break TagLoop
		case t == "c:members-only":
			accessType = accessTypeMemberOnly
			break TagLoop
		case t == "c:unlisted":
			accessType = accessTypeUnlisted
			break TagLoop
		case (t == "c:scheduled:hide" || t == "c:scheduled:show") && claim.Value.GetStream().ReleaseTime > time.Now().Unix():
			accessType = accessTypeScheduled
			break TagLoop
		}
	}

	if accessType == accessTypeFree {
		return true, nil
	}

	signErr := errNeedSignedUrl
	if isLivestream {
		signErr = errNeedSignedLivestreamUrl
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
			if purchase.Type == "rental" && time.Now().After(purchase.ValidThrough) {
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
		map[string]interface{}{
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
		return nil, errors.Err("couldn't find claim")
	}
	return &claim, err
}

func getStatusResponse(_ *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
	var response map[string]interface{}

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
	rpcResponse := GetQuery(ctx).newResponse()
	rpcResponse.Result = response
	return rpcResponse, nil
}

func expirationFunc(streamSize uint64) int64 {
	return paid.ExpTenSecPer100MB(streamSize) / 100 * 100
}
