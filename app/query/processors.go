package query

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/OdyseeTeam/odysee-api/internal/errors"

	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/player-server/pkg/paid"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/ybbus/jsonrpc"
)

var reAlreadyPurchased = regexp.MustCompile(`(?i)you already have a purchase`)
var rePurchaseFree = regexp.MustCompile(`(?i)does not have a purchase price`)
var controversialChannels = map[string]bool{
	"e23e889c6c0e06b3f59602061cee513067b506b6": true,
	"fdd11cb3ab75f95efb7b3bc2d726aa13ac915b66": true,
	"05155f4800fd11a21f83b8efc6446fa3d5d7b619": true,
	"9f638b94d11d879726ae55dd5a0923621b96a45b": true,
	"496730b88e41e74742fba5b6365435e5622dce92": true,
	"17765b4151a00937bb54c6bba5e1de4b64afb6eb": true,
	"a4a319a7d0282243b4b4d0a74f57dffe0bf1534b": true,
	"8c62bfde1569293cecdb3a24a09bb36786ca2b7c": true,
	"5754fdc9c5645fe8042f158709075b050ea58816": true,
	"2b9d66bc7db1f7587de778781aaf258e48db8595": true,
	"3abcd5048e9545393a9109525f55f8e5f0f919cd": true,
	"1da8dda83523b9836ddcd984d1963eff84102877": true,
	"a03414877cf3f059baebdf2e196ef127eecd5c2f": true,
	"d8cfc073e8cc2a5d28d8ad855b9975a5091afc3c": true,
	"ecfe66c4fdffb92e33229ab01371f97054890569": true,
	"8971c9199add01aa71f1bd060d2f9d6c4836fda0": true,
	"d12fe047afa550165916f1f361ed5c1fa3b45f50": true,
	"c1cdc9cbfd556fd0e80776a6d93ee5feddf46c92": true,
	"4d225e4db86f32151317ffd0002f4519906e083d": true,
	"ad919fcc26c31c081bd535aea60b580d1b783e5e": true,
	"0e8644a5d69058271dcdc816cf9cc917c69e0182": true,
	"73fc2d7baffc42090939ecc86cf05f3a835eac70": true,
	"aa36543e7eb68c64e76e39402c9cc4c9668368cd": true,
	"ad0458e5e2d1355744dc9b6dd9835d0f0afd9838": true,
	"b20e30a8375e58a77f6d276e08d9e2c81b8ba981": true,
	"e76f2d2cd12de2739be54b169a92bb39e7c24b17": true,
	"729a26df96109959e617e2eef80b5c2b590b5fba": true,
	"33513c58b93d0ef88b4a9195574f97863091a120": true,
	"bab918fdbb3821adf112d6a5f36b0d4676ebe7b7": true,
}

// preflightHookGet will completely replace `get` request from the client with `purchase_create` + `resolve`.
// This workaround is due to stability issues in the lbrynet SDK `get` method implementation.
// Only `ParamStreamingUrl` will be returned, plus `purchase_receipt` if stream has been paid for.
func preflightHookGet(caller *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
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
	var url string
	paramsMap := query.ParamsAsMap()
	uri, ok := paramsMap["uri"]
	if !ok {
		return nil, errors.Err("missing uri parameter for 'get' method")
	}
	url = uri.(string)
	log := logger.Log().WithField("url", url)

	claim, err := resolve(ctx, caller, query, url)
	if err != nil {
		return nil, err
	}
	stream := claim.Value.GetStream()

	feeAmount := stream.GetFee().GetAmount()
	if feeAmount > 0 {
		isPaidStream = true

		purchaseQuery, err := NewQuery(jsonrpc.NewRequest(
			MethodPurchaseCreate,
			map[string]interface{}{
				"url":      url,
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
			// Assuming the stream is of a paid variety and we have just paid for the stream
			metrics.LbrytvPurchases.Inc()
			metrics.LbrytvPurchaseAmounts.Observe(float64(feeAmount))
			log.Infof("made a purchase for %d LBC", feeAmount)
			// This is needed so changes can propagate for the subsequent resolve
			time.Sleep(1 * time.Second)
			claim, err = resolve(ctx, caller, query, url)
			if err != nil {
				return nil, err
			}
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
		return nil, fmt.Errorf(m)
	}
	sdHash := hex.EncodeToString(src.SdHash)[:6]
	if isPaidStream {
		size := src.GetSize()
		if claim.PurchaseReceipt == nil {
			log.Error("stream was paid for but receipt not found in the resolve response")
			return nil, fmt.Errorf("couldn't find purchase receipt for paid stream")
		}

		log.Debugf("creating stream token with stream id=%s, txid=%s, size=%v", claim.Name+"/"+claim.ClaimID, claim.PurchaseReceipt.Txid, size)
		token, err := paid.CreateToken(claim.Name+"/"+claim.ClaimID, claim.PurchaseReceipt.Txid, size, paid.ExpTenSecPer100MB)
		if err != nil {
			return nil, err
		}
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

func resolve(ctx context.Context, c *Caller, q *Query, url string) (*ljsonrpc.Claim, error) {
	resolveQuery, err := NewQuery(jsonrpc.NewRequest(
		MethodResolve,
		map[string]interface{}{
			"urls":                     url,
			"include_purchase_receipt": true,
			"include_protobuf":         true,
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
		return nil, fmt.Errorf("could not find a corresponding entry in the resolve response")
	}
	// Empty claim ID means that resolve error has been returned
	if claim.ClaimID == "" {
		return nil, fmt.Errorf("couldn't find claim")
	}
	return &claim, err
}

func getStatusResponse(c *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
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
