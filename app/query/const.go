package query

const (
	cacheResolveLongerThan = 10
	maxListSizeLogged      = 5

	// Method constants
	MethodAccountList          = "account_list"
	MethodClaimSearch          = "claim_search"
	MethodCommentReactList     = "comment_react_list"
	MethodFileList             = "file_list"
	MethodGet                  = "get"
	MethodPublish              = "publish"
	MethodPurchaseCreate       = "purchase_create"
	MethodResolve              = "resolve"
	MethodStatus               = "status"
	MethodStreamUpdate         = "stream_update"
	MethodStreamCreate         = "stream_create"
	MethodSyncApply            = "sync_apply"
	MethodWalletBalance        = "wallet_balance"
	MethodWalletSend           = "wallet_send"
	MethodBlobAnnounce         = "blob_announce"
	MethodTransactionShow      = "transaction_show"
	MethodStreamCostEstimate   = "stream_cost_estimate"
	MethodCommentList          = "comment_list"
	MethodCollectionResolve    = "collection_resolve"
	MethodVersion              = "version"
	MethodRoutingTableGet      = "routing_table_get"
	MethodAddressUnused        = "address_unused"
	MethodAddressList          = "address_list"
	MethodAddressIsMine        = "address_is_mine"
	MethodAccountBalance       = "account_balance"
	MethodAccountSend          = "account_send"
	MethodAccountMaxAddressGap = "account_max_address_gap"
	MethodChannelAbandon       = "channel_abandon"
	MethodChannelCreate        = "channel_create"
	MethodChannelList          = "channel_list"
	MethodChannelUpdate        = "channel_update"
	MethodChannelExport        = "channel_export"
	MethodChannelImport        = "channel_import"
	MethodChannelSign          = "channel_sign"
	MethodCollectionList       = "collection_list"
	MethodCollectionCreate     = "collection_create"
	MethodCollectionUpdate     = "collection_update"
	MethodCollectionAbandon    = "collection_abandon"
	MethodCommentAbandon       = "comment_abandon"
	MethodCommentCreate        = "comment_create"
	MethodCommentHide          = "comment_hide"
	MethodCommentUpdate        = "comment_update"
	MethodCommentReact         = "comment_react"
	MethodCommentPin           = "comment_pin"
	MethodClaimList            = "claim_list"
	MethodStreamAbandon        = "stream_abandon"
	MethodStreamRepost         = "stream_repost"
	MethodStreamList           = "stream_list"
	MethodSupportAbandon       = "support_abandon"
	MethodSupportCreate        = "support_create"
	MethodSupportList          = "support_list"
	MethodSyncHash             = "sync_hash"
	MethodPreferenceGet        = "preference_get"
	MethodPreferenceSet        = "preference_set"
	MethodPurchaseList         = "purchase_list"
	MethodTransactionList      = "transaction_list"
	MethodTxoList              = "txo_list"
	MethodTxoSpend             = "txo_spend"
	MethodTxoSum               = "txo_sum"
	MethodTxoPlot              = "txo_plot"
	MethodUtxoList             = "utxo_list"
	MethodUtxoRelease          = "utxo_release"
	MethodWalletList           = "wallet_list"
	MethodWalletEncrypt        = "wallet_encrypt"
	MethodWalletDecrypt        = "wallet_decrypt"
	MethodWalletLock           = "wallet_lock"
	MethodWalletUnlock         = "wallet_unlock"
	MethodWalletStatus         = "wallet_status"

	// Parameter constants
	ParamMethod           = "method"
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

var relaxedMethods = []string{
	MethodBlobAnnounce,
	MethodStatus,
	MethodResolve,
	MethodGet,
	MethodTransactionShow,
	MethodStreamCostEstimate,
	MethodClaimSearch,
	MethodCommentList,
	MethodCollectionResolve,
	MethodCommentReactList,
	MethodVersion,
	MethodRoutingTableGet,
}

var walletSpecificMethods = []string{
	MethodGet,
	MethodPurchaseCreate,
	MethodResolve,
	MethodClaimSearch,
	MethodPublish,
	MethodAddressUnused,
	MethodAddressList,
	MethodAddressIsMine,
	MethodAccountList,
	MethodAccountBalance,
	MethodAccountSend,
	MethodAccountMaxAddressGap,
	MethodChannelAbandon,
	MethodChannelCreate,
	MethodChannelList,
	MethodChannelUpdate,
	MethodChannelExport,
	MethodChannelImport,
	MethodChannelSign,
	MethodCollectionList,
	MethodCollectionCreate,
	MethodCollectionUpdate,
	MethodCollectionAbandon,
	MethodCommentAbandon,
	MethodCommentCreate,
	MethodCommentHide,
	MethodCommentUpdate,
	MethodCommentReact,
	MethodCommentPin,
	MethodCommentReactList,
	MethodClaimList,
	MethodStreamAbandon,
	MethodStreamCreate,
	MethodStreamList,
	MethodStreamUpdate,
	MethodStreamRepost,
	MethodSupportAbandon,
	MethodSupportCreate,
	MethodSupportList,
	MethodSyncApply,
	MethodSyncHash,
	MethodPreferenceGet,
	MethodPreferenceSet,
	MethodPurchaseList,
	MethodTransactionList,
	MethodTxoList,
	MethodTxoSpend,
	MethodTxoSum,
	MethodTxoPlot,
	MethodUtxoList,
	MethodUtxoRelease,
	MethodWalletList,
	MethodWalletSend,
	MethodWalletBalance,
	MethodWalletEncrypt,
	MethodWalletDecrypt,
	MethodWalletLock,
	MethodWalletUnlock,
	MethodWalletStatus,
}
