package query

import (
	"fmt"

	"github.com/lbryio/lbrytv/app/paid"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/errors"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	log "github.com/sirupsen/logrus"
	"github.com/ybbus/jsonrpc"
)

func postProcessResponse(response *jsonrpc.RPCResponse, query *jsonrpc.RPCRequest) error {
	switch query.Method {
	case MethodGet:
		return responseProcessorGet(response, query)
	case MethodFileList:
		return responseProcessorFileList(response)
	case MethodAccountList:
		return responseProcessorAccountList(response, query)
	default:
		return nil
	}
}

func responseProcessorGet(response *jsonrpc.RPCResponse, query *jsonrpc.RPCRequest) error {
	var result map[string]interface{}
	var txid string
	var urlSuffix string

	baseGetResponse := &ljsonrpc.GetResponse{}

	err := response.GetObject(&result)
	if err != nil {
		return err
	}

	// If content_fee is present we assume it's paid for, otherwise it's a free stream
	if cFee, ok := result["content_fee"]; ok {
		if cF, ok := cFee.(map[string]interface{}); ok {
			txid = cF["txid"].(string)
		}
	}

	err = ljsonrpc.Decode(result, baseGetResponse)
	if err != nil {
		return err
	}

	if txid != "" {
		token, err := paid.CreateToken(baseGetResponse.SdHash, txid, baseGetResponse.Metadata.GetStream().Source.Size, paid.ExpTenSecPer100MB)
		if err != nil {
			return err
		}
		urlSuffix = fmt.Sprintf("paid/%s/%s", baseGetResponse.SdHash, token)
	} else {
		urlSuffix = fmt.Sprintf("free/%s", baseGetResponse.SdHash)
	}

	result["streaming_url"] = config.GetConfig().Viper.GetString("BaseContentURL") + urlSuffix
	response.Result = result
	return nil
}

func responseProcessorFileList(response *jsonrpc.RPCResponse) error {
	var resultArray []map[string]interface{}
	err := response.GetObject(&resultArray)
	if err != nil {
		return err
	}

	if len(resultArray) != 0 {
		resultArray[0]["download_path"] = fmt.Sprintf(
			"%sclaims/%s/%s/%s",
			config.GetConfig().Viper.GetString("BaseContentURL"),
			resultArray[0]["claim_name"], resultArray[0]["claim_id"],
			resultArray[0]["file_name"])
	}

	response.Result = resultArray
	return nil
}

func responseProcessorAccountList(response *jsonrpc.RPCResponse, query *jsonrpc.RPCRequest) error {
	logger.WithFields(log.Fields{"params": query.Params}).Info("got account_list query")

	if query.Params == nil {
		accounts := new(ljsonrpc.AccountListResponse)
		// No account_id is supplied, get the default account and return it
		ljsonrpc.Decode(response.Result, accounts)
		account := getDefaultAccount(accounts)
		if account == nil {
			return errors.Err("fatal error: no default account found")
		}
		response.Result = account
	}

	return nil
}

func getDefaultAccount(accounts *ljsonrpc.AccountListResponse) *ljsonrpc.Account {
	for _, account := range accounts.Items {
		if account.IsDefault {
			return &account
		}
	}
	return nil
}
