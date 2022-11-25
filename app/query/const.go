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
	MethodStreamCreate     = "stream_create"
	MethodSyncApply        = "sync_apply"
	MethodWalletBalance    = "wallet_balance"
	MethodWalletSend       = "wallet_send"

	ParamAccountID        = "account_id"
	ParamChannelID        = "channel_id"
	ParamNewSDKServer     = "new_sdk_server"
	ParamPurchaseReceipt  = "purchase_receipt"
	ParamStreamingUrl     = "streaming_url"
	ParamBaseStreamingUrl = "base_streaming_url"
	ParamUrls             = "urls"
	ParamWalletID         = "wallet_id"
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
	MethodStreamCreate,
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
	"a6a125c99ba2a6c068a8b502fb08dda603f6f03a": true,
	"fe6e65a6d69c9093cb0482d4d098598ec4403d61": true,
	"2f98945fec7687d7ad2f8a47f88b1875ce102ca3": true,
	"f73d3d42ae8c4f2cd347a81917a505340ef5fe47": true,
	"c7aa6ca3ba6b0f17ffa83c8c8e69049908052cd8": true,
	"40b9c2158d9c59b9ef8ee6998915802b021e4ac8": true,
	"6d895d997ad67fe61dc08cbaa43a2df2fc89508b": true,
	"22c4cd79217ffc56078c4a17f49ccc73d1693ad3": true,
}
