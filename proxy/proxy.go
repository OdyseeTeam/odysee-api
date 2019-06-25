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
	"github.com/lbryio/lbrytv/monitor"

	log "github.com/sirupsen/logrus"
	"github.com/ybbus/jsonrpc"
)

const cacheResolveLongerThan = 10

var accountSpecificMethods = []string{
	"publish",
	"account_list",
	"account_balance",
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

// ErrProxy is for general errors that originate inside the proxy module
const ErrProxy int = -32080

// ErrProxyAuthFailed is when supplied auth_token / account_id is not present in the database
const ErrProxyAuthFailed int = -32085

// ErrInternal is a general server error code
const ErrInternal int = -32603

// ErrInvalidParams signifies a client-supplied params error
const ErrInvalidParams int = -32602

// ErrMethodNotFound means the client-requested method cannot be found
const ErrMethodNotFound int = -32601

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
		monitor.Logger.WithFields(log.Fields{
			"method": r.Method, "params": r.Params,
		}).Info("got a preconditioned query response")
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
	} else {
		return *jsonrpc.NewRequest(method)
	}
}

// RawCall makes an arbitrary jsonrpc request to the SDK
func RawCall(request jsonrpc.RPCRequest) (*jsonrpc.RPCResponse, error) {
	rpcClient := jsonrpc.NewClient(config.Settings.GetString("Lbrynet"))
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
		processedResponse, err = processResponse(&request, callResult)
		if err != nil {
			return nil, err
		}
		// There will be too many account_balance requests, we don't need to log them
		if request.Method != "account_balance" {
			monitor.LogSuccessfulQuery(request.Method, time.Now().Sub(queryStartTime).Seconds())
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
	if method == "resolve" && params != nil {
		paramsMap := params.(map[string]interface{})
		if urls, ok := paramsMap["urls"].([]interface{}); ok {
			if len(urls) > cacheResolveLongerThan {
				return true
			}
		}
	}
	return false
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
