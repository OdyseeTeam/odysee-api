package query

const (
	cacheResolveLongerThan = 10
	maxListSizeLogged      = 5

	MethodAccountList      = "account_list"
	MethodClaimSearch      = "claim_search"
	MethodCommentReactList = "comment_react_list"
	MethodFileList         = "file_list"
	MethodGet              = "get"
	MethodPublish          = "publish"
	MethodPurchaseCreate   = "purchase_create"
	MethodResolve          = "resolve"
	MethodStatus           = "status"
	MethodStreamUpdate     = "stream_update"
	MethodSyncApply        = "sync_apply"
	MethodWalletBalance    = "wallet_balance"
	MethodWalletSend       = "wallet_send"

	ParamAccountID       = "account_id"
	ParamChannelID       = "channel_id"
	ParamNewSDKServer    = "new_sdk_server"
	ParamPurchaseReceipt = "purchase_receipt"
	ParamStreamingUrl    = "streaming_url"
	ParamUrls            = "urls"
	ParamWalletID        = "wallet_id"
)

var forbiddenParams = []string{ParamAccountID, ParamNewSDKServer}

// relaxedMethods are methods which are allowed to be called without wallet_id.
var relaxedMethods = []string{
	"blob_announce",
	MethodStatus,
	MethodResolve,
	MethodGet,
	"transaction_show",
	"stream_cost_estimate",
	"claim_search",
	"comment_list",
	"collection_resolve",
	MethodCommentReactList,
	"version",
	"routing_table_get",
}

// walletSpecificMethods are methods which require wallet_id.
// This list will inevitably turn stale sooner or later as new methods
// are added to the SDK so relaxedMethods should be used for strict validation
// whether wallet_id is required.
var walletSpecificMethods = []string{
	MethodGet,
	MethodPurchaseCreate,

	"resolve",
	"claim_search",

	MethodPublish,

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
	"channel_sign",

	"collection_list",
	"collection_create",
	"collection_update",
	"collection_abandon",

	"comment_abandon",
	"comment_create",
	"comment_hide",
	"comment_update",
	"comment_react",
	"comment_pin",
	MethodCommentReactList,

	"claim_list",

	"stream_abandon",
	"stream_create",
	"stream_list",
	MethodStreamUpdate,
	"stream_repost",

	"support_abandon",
	"support_create",
	"support_list",

	MethodSyncApply,
	"sync_hash",

	"preference_get",
	"preference_set",

	"purchase_list",

	"transaction_list",

	"txo_list",
	"txo_spend",
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
