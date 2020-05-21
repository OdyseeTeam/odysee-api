package query

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/lbryio/lbrytv-player/pkg/paid"
	"github.com/lbryio/lbrytv/config"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/ybbus/jsonrpc"
)

var reAlreadyPurchased = regexp.MustCompile(`(?i)you already have a purchase`)
var rePurchaseFree = regexp.MustCompile(`(?i)does not have a purchase price`)

func preflightHookGet(caller *Caller, query *Query) (*jsonrpc.RPCResponse, error) {
	var urlSuffix string
	var isPaidStream bool

	response := &jsonrpc.RPCResponse{
		ID:      query.Request.ID,
		JSONRPC: query.Request.JSONRPC,
	}
	responseResult := map[string]interface{}{
		"streaming_url": "UNSET",
	}

	url := query.ParamsAsMap()["uri"].(string)
	log := logger.Log().WithField("url", url)

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
	purchaseRes, err := caller.callQueryWithRetry(purchaseQuery)
	if err != nil {
		return nil, err
	}
	if purchaseRes.Error != nil {
		if reAlreadyPurchased.MatchString(purchaseRes.Error.Message) {
			log.Debug("purchase_create says stream is already purchased")
			isPaidStream = true
		} else if rePurchaseFree.MatchString(purchaseRes.Error.Message) {
			log.Debug("purchase_create says stream is free")
			isPaidStream = false
		} else {
			return nil, fmt.Errorf("purchase error: %v", purchaseRes.Error.Message)
		}
	} else {
		// Assuming the stream is of a paid variety and we have paid for the stream
		isPaidStream = true
		// This is needed so changes can propagate for the subsequent resolve
		time.Sleep(1 * time.Second)
	}

	resolveResponse := ljsonrpc.ResolveResponse{}

	resQuery, err := NewQuery(jsonrpc.NewRequest(
		MethodResolve,
		map[string]interface{}{
			"urls":                     url,
			"include_purchase_receipt": true,
			"include_protobuf":         true,
		},
	), query.WalletID)
	if err != nil {
		return nil, err
	}

	resRespRaw, err := caller.callQueryWithRetry(resQuery)
	if err != nil {
		return nil, err
	}

	resResult := map[string]interface{}{}
	err = resRespRaw.GetObject(&resResult)
	if err != nil {
		log.Debug("error parsing resolve response:", err)
		return nil, err
	}
	err = ljsonrpc.Decode(resResult, &resolveResponse)
	if err != nil {
		return nil, err
	}

	claim, ok := resolveResponse[url]
	if !ok {
		return nil, fmt.Errorf("could not find a corresponding entry in the resolve response")
	}

	stream := claim.Value.GetStream()
	size := stream.GetSource().Size

	if err != nil {
		return nil, fmt.Errorf("error getting size by magic: %v", err)
	}

	if isPaidStream {
		if claim.PurchaseReceipt == nil {
			log.Errorf("stream was paid for but receipt not found in the resolve response")
			return nil, fmt.Errorf("couldn't find purchase receipt for paid stream")
		}

		log.Debugf("creating stream token with stream id=%s, txid=%s, size=%v", claim.Name+"/"+claim.ClaimID, claim.PurchaseReceipt.Txid, uint64(size))
		token, err := paid.CreateToken(claim.Name+"/"+claim.ClaimID, claim.PurchaseReceipt.Txid, uint64(size), paid.ExpTenSecPer100MB)
		if err != nil {
			return nil, err
		}
		urlSuffix = fmt.Sprintf("paid/%s/%s/%s", claim.Name, claim.ClaimID, token)
		responseResult["purchase_receipt"] = claim.PurchaseReceipt
	} else {
		urlSuffix = fmt.Sprintf("free/%s/%s", claim.Name, claim.ClaimID)
	}

	responseResult["streaming_url"] = config.GetConfig().Viper.GetString("BaseContentURL") + urlSuffix

	response.Result = responseResult
	return response, nil
}

func checkIsPaidStream(s interface{}) bool {
	f := s.(ljsonrpc.File)
	stream := f.Metadata.GetStream()
	return stream.Fee != nil && stream.Fee.Amount > 0
}

func getStatusResponse(c *Caller, q *Query) (*jsonrpc.RPCResponse, error) {
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
	rpcResponse := q.newResponse()
	rpcResponse.Result = response
	return rpcResponse, nil
}
