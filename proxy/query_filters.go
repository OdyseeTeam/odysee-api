package proxy

import (
	"encoding/json"
	"fmt"

	"github.com/ybbus/jsonrpc"
)

var forbiddenMethods = []string{
	"stop",
	"account_add",
	"account_create",
	"account_encrypt",
	"account_decrypt",
	"account_fund",
	// "account_list",
	"account_lock",
	"account_remove",
	"account_set",
	"account_unlock",
	"get",
	"sync_apply",
}

const forbiddenParam = "account_id"

func MethodRequiresAccountID(method string) bool {
	return methodInList(method, accountSpecificMethods)
}

func methodInList(method string, checkMethods []string) bool {
	for _, m := range checkMethods {
		if m == method {
			return true
		}
	}
	return false
}

// getPreconditionedQueryResponse returns true if we got a resolve query with more than `cacheResolveLongerThan` urls in it
func getPreconditionedQueryResponse(method string, params interface{}) *jsonrpc.RPCResponse {
	if methodInList(method, forbiddenMethods) {
		return NewErrorResponse(fmt.Sprintf("Forbidden method requested: %v", method), ErrMethodNotFound)
	}

	if paramsMap, ok := params.(map[string]interface{}); ok {
		if _, ok := paramsMap[forbiddenParam]; ok {
			return NewErrorResponse(fmt.Sprintf("Forbidden parameter supplied: %v", forbiddenParam), ErrInvalidParams)
		}
	}

	if method == methodStatus {
		var r jsonrpc.RPCResponse
		r.Result = getStatusResponse()
		return &r
	}
	return nil
}

func getStatusResponse() map[string]interface{} {
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
		"installation_id": "lbrytv",
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
	return response
}
