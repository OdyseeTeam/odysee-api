// Package proxy handles incoming JSON-RPC requests from UI client (lbry-desktop or any other), forwards them to the actual SDK instance running nearby and returns its response to the client.
// The purpose of it is to expose SDK over a publicly accessible http interface,  fixing aspects of it which normally would prevent SDK from being safely or efficiently shared between multiple remote clients.

// Currently it does:

// * Request validation
// * Request processing
// * Gatekeeping (blocks certain methods from being called)
// * Response processing
// * Response caching

package proxy

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/monitor"

	log "github.com/sirupsen/logrus"
	"github.com/ybbus/jsonrpc"
)

const cacheResolveLongerThan = 10

var accountSpecificMethods = []string{
	"publish",

	"address_unused",
	"address_list",
	"address_is_mine",

	"account_list",
	"account_balance",
	"account_send",

	"channel_abandon",
	"channel_create",
	"channel_list",
	"channel_update",

	"claim_list",

	"stream_abandon",
	"stream_create",
	"stream_list",
	"stream_update",

	"support_abandon",
	"support_create",
	"support_list",

	"transaction_list",

	"utxo_list",
	"utxo_release",
}

var accountFundingSpecificMethods = []string{
	"channel_create",
	"channel_update",
	"publish",
	"stream_create",
	"stream_update",
	"support_create",
}

const MethodGet = "get"
const MethodFileList = "file_list"
const MethodAccountList = "account_list"
const MethodAccountBalance = "account_balance"
const MethodStatus = "status"
const MethodResolve = "resolve"
const MethodClaimSearch = "claim_search"
const MethodCommentList = "comment_list"

const paramAccountID = "account_id"
const paramWalletID = "wallet_id"
const paramFundingAccountIDs = "funding_account_ids"
const paramUrls = "urls"

var ignoreLog = []string{
	MethodAccountBalance,
	MethodStatus,
}

var ResolveTime float64

// UnmarshalRequest takes a raw json request body and serializes it into RPCRequest struct for further processing.
func UnmarshalRequest(r []byte) (*jsonrpc.RPCRequest, error) {
	var ur jsonrpc.RPCRequest
	err := json.Unmarshal(r, &ur)
	if err != nil {
		return nil, fmt.Errorf("client json parse error: %v", err)
	}
	return &ur, nil
}

// Proxy takes a parsed jsonrpc request, calls processors on it and passes it over to the daemon.
// If accountID is supplied, it's injected as a request param.
func Proxy(r *jsonrpc.RPCRequest, accountID string) ([]byte, error) {
	resp := preprocessRequest(r, accountID)
	if resp != nil {
		return MarshalResponse(resp)
	}
	return ForwardCall(*r)
}

// MarshalResponse takes a raw json request body and serializes it into RPCRequest struct for further processing.
func MarshalResponse(r *jsonrpc.RPCResponse) ([]byte, error) {
	sr, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return sr, nil
}

// NewErrorResponse is a shorthand for creating an RPCResponse instance with specified error message and code
func NewErrorResponse(message string, code int) *jsonrpc.RPCResponse {
	return &jsonrpc.RPCResponse{Error: &jsonrpc.RPCError{
		Code:    code,
		Message: message,
	}}
}

func preprocessRequest(r *jsonrpc.RPCRequest, accountID string) *jsonrpc.RPCResponse {
	var resp *jsonrpc.RPCResponse

	resp = getPreconditionedQueryResponse(r.Method, r.Params)
	if resp != nil {
		return resp
	}

	if accountID != "" && methodInList(r.Method, accountSpecificMethods) {
		monitor.Logger.WithFields(log.Fields{
			"method": r.Method, "params": r.Params,
		}).Info("got an account-specific method call")

		if paramsMap, ok := r.Params.(map[string]interface{}); ok {
			paramsMap["account_id"] = accountID
			r.Params = paramsMap
		} else {
			r.Params = map[string]string{"account_id": accountID}
		}
	}
	processQuery(r)

	if shouldCache(r.Method, r.Params) {
		cResp := responseCache.Retrieve(r.Method, r.Params)
		if cResp != nil {
			// TODO: Temporary hack to find out why the following line doesn't work
			// if mResp, ok := cResp.(map[string]interface{}); ok {
			s, _ := json.Marshal(cResp)
			err := json.Unmarshal(s, &resp)
			if err == nil {
				resp.ID = r.ID
				resp.JSONRPC = r.JSONRPC
				monitor.LogCachedQuery(r.Method)
				return resp
			}
		}
	}
	return resp
}

func NewRequest(method string, params ...interface{}) jsonrpc.RPCRequest {
	if len(params) > 0 {
		return *jsonrpc.NewRequest(method, params[0])
	}
	return *jsonrpc.NewRequest(method)
}

// RawCall makes an arbitrary jsonrpc request to the SDK
func RawCall(request jsonrpc.RPCRequest) (*jsonrpc.RPCResponse, error) {
	rpcClient := jsonrpc.NewClient(config.GetLbrynet())
	response, err := rpcClient.CallRaw(&request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// ForwardCall passes a prepared jsonrpc request to the SDK and calls post-processors on the response.
func ForwardCall(request jsonrpc.RPCRequest) ([]byte, error) {
	var processedResponse *jsonrpc.RPCResponse
	queryStartTime := time.Now()
	callResult, err := RawCall(request)
	if err != nil {
		return nil, err
	}
	if callResult.Error == nil {
		execTime := time.Now().Sub(queryStartTime).Seconds()

		processedResponse, err = processResponse(&request, callResult)
		if err != nil {
			return nil, err
		}

		if shouldLog(request.Method) {
			monitor.LogSuccessfulQuery(request.Method, execTime, request.Params)
		}

		if request.Method == "resolve" {
			ResolveTime = execTime
		}

		if shouldCache(request.Method, request.Params) {
			responseCache.Save(request.Method, request.Params, processedResponse)
		}
	} else {
		processedResponse = callResult
		monitor.LogFailedQuery(request.Method, request.Params, callResult.Error)
	}

	resp, err := MarshalResponse(processedResponse)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// shouldCache returns true if we got a resolve query with more than `cacheResolveLongerThan` urls in it.
func shouldCache(method string, params interface{}) bool {
	if method == MethodResolve && params != nil {
		paramsMap := params.(map[string]interface{})
		if urls, ok := paramsMap[paramUrls].([]interface{}); ok {
			if len(urls) > cacheResolveLongerThan {
				return true
			}
		}
	}
	return false
}

func shouldLog(method string) bool {
	for _, m := range ignoreLog {
		if m == method {
			return false
		}
	}
	return true
}

func getQueryParams(query *jsonrpc.RPCRequest) (queryParams map[string]interface{}, err error) {
	stringifiedParams, err := json.Marshal(query.Params)
	if err != nil {
		return nil, err
	}

	queryParams = map[string]interface{}{}
	err = json.Unmarshal(stringifiedParams, &queryParams)
	if err != nil {
		return nil, err
	}
	return
}
