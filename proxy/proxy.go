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

/*
ForwardCall takes a raw client request, passes it over to the daemon and returns daemon response.

Example:

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Panicf("error: ", err.Error())
	}
	lbrynetResponse, err := proxy.ForwardCall(body)
*/
func ForwardCall(rawRequest []byte) ([]byte, error) {
	var request jsonrpc.RPCRequest
	var processedResponse *jsonrpc.RPCResponse
	rpcClient := jsonrpc.NewClient(config.Settings.GetString("Lbrynet"))

	err := json.Unmarshal(rawRequest, &request)
	if err != nil {
		return nil, fmt.Errorf("client json parse error: %v", err)
	}

	pqr := getPreconditionedQueryResponse(request.Method, request.Params)
	if pqr != nil {
		monitor.Logger.WithFields(log.Fields{
			"method": request.Method, "params": request.Params,
		}).Info("got a preconditioned query response")
		serializedResponse, err := json.MarshalIndent(pqr, "", "  ")
		if err != nil {
			return nil, err
		}
		return serializedResponse, nil
	}

	finalQuery, err := processQuery(&request)
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
			monitor.LogCachedQuery(request.Method)
			return serializedResponse, nil
		}
	}

	queryStartTime := time.Now()
	callResult, err := rpcClient.CallRaw(finalQuery)
	if err != nil {
		return nil, err
	}
	if callResult.Error == nil {
		processedResponse, err = processResponse(&request, callResult)
		if err != nil {
			return nil, err
		}
		// Too many account_balance requests, no need to log them
		if finalQuery.Method != "account_balance" {
			monitor.LogSuccessfulQuery(request.Method, time.Now().Sub(queryStartTime).Seconds())
		}
		if shouldCache(finalQuery.Method, finalQuery.Params) {
			responseCache.Save(finalQuery.Method, finalQuery.Params, processedResponse)
		}
	} else {
		processedResponse = callResult
		monitor.LogFailedQuery(request.Method, request.Params, callResult.Error)
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
