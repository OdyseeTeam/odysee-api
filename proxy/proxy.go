package proxy

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lbryio/lbryweb.go/config"
	"github.com/lbryio/lbryweb.go/monitor"
	"github.com/ybbus/jsonrpc"
)

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

	finalQuery, err := processQuery(&parsedClientQuery)
	if err != nil {
		return nil, err
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
		if parsedClientQuery.Method != "account_balance" {
			monitor.LogSuccessfulQuery(parsedClientQuery.Method, time.Now().Sub(queryStartTime).Seconds())
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

func processQuery(query *jsonrpc.RPCRequest) (processedQuery *jsonrpc.RPCRequest, err error) {
	processedQuery = query
	switch query.Method {
	case "get":
		processedQuery, err = getQueryProcessor(query)
	}
	return processedQuery, err
}

func processResponse(query *jsonrpc.RPCRequest, response *jsonrpc.RPCResponse) (processedResponse *jsonrpc.RPCResponse, err error) {
	processedResponse = response
	switch query.Method {
	case "get":
		processedResponse, err = getResponseProcessor(query, response)
	case "file_list":
		processedResponse, err = fileListResponseProcessor(query, response)
	}
	return processedResponse, err
}

func getQueryProcessor(query *jsonrpc.RPCRequest) (*jsonrpc.RPCRequest, error) {
	return query, nil
}

func getResponseProcessor(query *jsonrpc.RPCRequest, response *jsonrpc.RPCResponse) (*jsonrpc.RPCResponse, error) {
	var err error
	result := map[string]interface{}{}
	response.GetObject(&result)

	stringifiedParams, err := json.Marshal(query.Params)
	if err != nil {
		return response, err
	}

	queryParams := map[string]interface{}{}
	err = json.Unmarshal(stringifiedParams, &queryParams)
	if err != nil {
		return response, err
	}
	result["download_path"] = fmt.Sprintf(
		"%s%s/%s", config.Settings.GetString("BaseContentURL"), queryParams["uri"], result["outpoint"])
	response.Result = result
	return response, nil
}

func fileListResponseProcessor(query *jsonrpc.RPCRequest, response *jsonrpc.RPCResponse) (*jsonrpc.RPCResponse, error) {
	var err error
	var resultArray []map[string]interface{}
	response.GetObject(&resultArray)

	if err != nil {
		return response, err
	}

	if len(resultArray) != 0 {
		resultArray[0]["download_path"] = fmt.Sprintf(
			"%sclaims/%s/%s/%s",
			config.Settings.GetString("BaseContentURL"),
			resultArray[0]["claim_name"], resultArray[0]["claim_id"],
			resultArray[0]["file_name"])
	}
	response.Result = resultArray
	return response, nil
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
