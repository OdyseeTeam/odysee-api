package proxy

import (
	"encoding/json"
	"fmt"

	"github.com/lbryio/lbrytv/config"
	"github.com/ybbus/jsonrpc"
)

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
