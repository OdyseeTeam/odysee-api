package query

import (
	"encoding/json"
)

const (
	cacheResolveLongerThan = 10
	forbiddenParam         = paramAccountID

	paramAccountID = "account_id"
	paramWalletID  = "wallet_id"
	paramUrls      = "urls"

	MethodGet         = "get"
	MethodFileList    = "file_list"
	MethodAccountList = "account_list"
	MethodStatus      = "status"
	MethodResolve     = "resolve"
	MethodClaimSearch = "claim_search"
)

// relaxedMethods are methods which are allowed to be called without wallet_id.
var relaxedMethods = []string{
	"blob_announce",
	"status",
	"resolve",
	"transaction_show",
	"stream_cost_estimate",
	"claim_search",
	"comment_list",
	"version",
	"routing_table_get",
}

// walletSpecificMethods are methods which require wallet_id.
// This list will inevitably turn stale sooner or later as new methods
// are added to the SDK so relaxedMethods should be used for strict validation
// whether wallet_id is required.
var walletSpecificMethods = []string{
	MethodGet,

	"resolve",
	"claim_search",

	"publish",

	"address_unused",
	"address_list",
	"address_is_mine",

	"account_list",
	"account_balance",
	"account_send",
	"account_max_address_gap",

	"channel_abandon",
	"channel_create",
	"channel_list",
	"channel_update",
	"channel_export",
	"channel_import",

	"comment_abandon",
	"comment_create",
	"comment_hide",
	"comment_update",

	"claim_list",

	"stream_abandon",
	"stream_create",
	"stream_list",
	"stream_update",
	"stream_repost",

	"support_abandon",
	"support_create",
	"support_list",

	"sync_apply",
	"sync_hash",

	"preference_get",
	"preference_set",

	"transaction_list",

	"txo_list",
	"txo_sum",
	"txo_plot",

	"utxo_list",
	"utxo_release",

	"wallet_list",
	"wallet_send",
	"wallet_balance",
	"wallet_encrypt",
	"wallet_decrypt",
	"wallet_lock",
	"wallet_unlock",
	"wallet_status",
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
	return response
}
