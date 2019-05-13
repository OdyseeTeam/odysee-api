package proxy

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/monitor"
	"github.com/ybbus/jsonrpc"
)

const cacheResolveLongerThan = 10

/*
ForwardCall takes a raw client request, passes it over to the daemon and returns daemon response.

Example:

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Panicf("error: ", err.Error())
	}
	lbrynetResponse, err := proxy.ForwardCall(body)
*/
func ForwardCall(clientQuery []byte) ([]byte, error) {
	var parsedClientQuery jsonrpc.RPCRequest
	var processedResponse *jsonrpc.RPCResponse
	rpcClient := jsonrpc.NewClient(config.Settings.GetString("Lbrynet"))

	err := json.Unmarshal(clientQuery, &parsedClientQuery)
	if err != nil {
		return nil, fmt.Errorf("client json parse error: %v", err)
	}

	pqr := getPreconditionedQueryResponse(parsedClientQuery.Method, parsedClientQuery.Params)
	if pqr != nil {
		serializedResponse, err := json.MarshalIndent(pqr, "", "  ")
		if err != nil {
			return nil, err
		}
		return serializedResponse, nil
	}

	finalQuery, err := processQuery(&parsedClientQuery)
	if err != nil {
		return nil, err
	}

	if shouldCache(finalQuery.Method, finalQuery.Params) {
		cachedResponse := responseCache.Retrieve(finalQuery.Method, finalQuery.Params)
		if cachedResponse != nil {
			serializedResponse, err := json.MarshalIndent(cachedResponse, "", "  ")
			if err != nil {
				return nil, err
			}
			monitor.LogCachedQuery(parsedClientQuery.Method)
			return serializedResponse, nil
		}
	}

	queryStartTime := time.Now()
	callResult, err := rpcClient.CallRaw(finalQuery)
	if err != nil {
		return nil, err
	}
	if callResult.Error == nil {
		processedResponse, err = processResponse(&parsedClientQuery, callResult)
		if err != nil {
			return nil, err
		}
		// Too many account_balance requests, no need to log them
		if finalQuery.Method != "account_balance" {
			monitor.LogSuccessfulQuery(parsedClientQuery.Method, time.Now().Sub(queryStartTime).Seconds())
		}
		if shouldCache(finalQuery.Method, finalQuery.Params) {
			responseCache.Save(finalQuery.Method, finalQuery.Params, processedResponse)
		}
	} else {
		processedResponse = callResult
		monitor.LogFailedQuery(parsedClientQuery.Method, parsedClientQuery.Params, callResult.Error)
	}

	serializedResponse, err := json.MarshalIndent(processedResponse, "", "  ")

	if err != nil {
		return nil, err
	}
	return serializedResponse, nil
}

// shouldCache returns true if we got a resolve query with more than `cacheResolveLongerThan` urls in it
func shouldCache(method string, params interface{}) bool {
	if method == "resolve" {
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
