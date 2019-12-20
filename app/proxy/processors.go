package proxy

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/monitor"
	log "github.com/sirupsen/logrus"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/ybbus/jsonrpc"
)

func processQuery(query *jsonrpc.RPCRequest) (processedQuery *jsonrpc.RPCRequest, err error) {
	processedQuery = query
	switch query.Method {
	case MethodGet:
		processedQuery, err = queryProcessorGet(query)
	}
	return processedQuery, err
}

func processResponse(query *jsonrpc.RPCRequest, response *jsonrpc.RPCResponse) (processedResponse *jsonrpc.RPCResponse, err error) {
	processedResponse = response
	switch query.Method {
	case MethodGet:
		processedResponse, err = responseProcessorGet(query, response)
	case MethodFileList:
		processedResponse, err = responseProcessorFileList(query, response)
	case MethodAccountList:
		processedResponse, err = responseProcessorAccountList(query, response)
	}
	return processedResponse, err
}

func queryProcessorGet(query *jsonrpc.RPCRequest) (*jsonrpc.RPCRequest, error) {
	return query, nil
}

func responseProcessorGet(query *jsonrpc.RPCRequest, response *jsonrpc.RPCResponse) (*jsonrpc.RPCResponse, error) {
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
		"%s%s/%s", config.GetConfig().Viper.GetString("BaseContentURL"), queryParams["uri"], result["outpoint"])
	response.Result = result
	return response, nil
}

func responseProcessorFileList(query *jsonrpc.RPCRequest, response *jsonrpc.RPCResponse) (*jsonrpc.RPCResponse, error) {
	var err error
	var resultArray []map[string]interface{}
	response.GetObject(&resultArray)

	if err != nil {
		return response, err
	}

	if len(resultArray) != 0 {
		resultArray[0]["download_path"] = fmt.Sprintf(
			"%sclaims/%s/%s/%s",
			config.GetConfig().Viper.GetString("BaseContentURL"),
			resultArray[0]["claim_name"], resultArray[0]["claim_id"],
			resultArray[0]["file_name"])
	}
	response.Result = resultArray
	return response, nil
}

func getDefaultAccount(accounts *ljsonrpc.AccountListResponse) *ljsonrpc.Account {
	for _, account := range accounts.Items {
		if account.IsDefault {
			return &account
		}
	}
	return nil
}

func responseProcessorAccountList(query *jsonrpc.RPCRequest, response *jsonrpc.RPCResponse) (*jsonrpc.RPCResponse, error) {
	accounts := new(ljsonrpc.AccountListResponse)
	// result := map[string]interface{}{}
	// response.GetObject(&result)

	monitor.Logger.WithFields(log.Fields{
		"params": query.Params,
	}).Info("got account_list query")
	if query.Params == nil {
		// No account_id is supplied, get the default account and return it
		ljsonrpc.Decode(response.Result, accounts)
		account := getDefaultAccount(accounts)
		if account == nil {
			return nil, errors.New("fatal error: no default account found")
		}
		response.Result = account
	}
	// response.Result = result
	return response, nil
}
