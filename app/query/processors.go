package query

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/lbryio/lbrytv/internal/errors"

	"github.com/lbryio/lbrytv-player/pkg/paid"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/ybbus/jsonrpc"
)

var reAlreadyPurchased = regexp.MustCompile(`(?i)you already have a purchase`)
var rePurchaseFree = regexp.MustCompile(`(?i)does not have a purchase price`)

// preflightHookGet will completely replace `get` request from the client with `purchase_create` + `resolve`.
// This workaround is due to stability issues in the lbrynet SDK `get` method implementation.
// Only `ParamStreamingUrl` will be returned, plus `purchase_receipt` if stream has been paid for.
func preflightHookGet(caller *Caller, hctx *HookContext) (*jsonrpc.RPCResponse, error) {
	var (
		contentURL, metricLabel string
		isPaidStream            bool
	)
	query := hctx.Query

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

	claim, err := resolve(caller, query, url)
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
		purchaseRes, err := caller.SendQuery(purchaseQuery)
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
			claim, err = resolve(caller, query, url)
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
		contentURL = fmt.Sprintf(
			"%v%s/%s/%s/%s",
			config.Config.Viper.GetString("PaidContentURL"), claim.Name, claim.ClaimID, sdHash, token)
		responseResult[ParamPurchaseReceipt] = claim.PurchaseReceipt
	} else {
		contentURL = fmt.Sprintf(
			"%v%s/%s/%s",
			config.Config.Viper.GetString("FreeContentURL"), claim.Name, claim.ClaimID, sdHash)
	}

	responseResult[ParamStreamingUrl] = contentURL

	response.Result = responseResult
	return response, nil
}

func resolve(c *Caller, q *Query, url string) (*ljsonrpc.Claim, error) {
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

	rawResolveResponse, err := c.SendQuery(resolveQuery)
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

func getStatusResponse(c *Caller, hctx *HookContext) (*jsonrpc.RPCResponse, error) {
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
	rpcResponse := hctx.Query.newResponse()
	rpcResponse.Result = response
	return rpcResponse, nil
}
