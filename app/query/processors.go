package query

import (
	"fmt"

	"github.com/lbryio/lbrytv-player/pkg/paid"
	"github.com/lbryio/lbrytv/config"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/ybbus/jsonrpc"
)

type PurchaseReceipt struct {
	Address       string `json:"file_name"`
	Amount        string `json:"amount"`
	ClaimID       string `json:"claim_id"`
	Confirmations int    `json:"confirmations"`
	Height        int    `json:"height"`
	Nout          uint64 `json:"nout"`
	Timestamp     uint64 `json:"timestamp"`
	Txid          string `json:"txid"`
	Type          string `json:"purchase"`
}

func postProcessResponse(caller *Caller, query *Query, response *jsonrpc.RPCResponse) error {
	switch query.Method() {
	case MethodGet:
		return responseProcessorGet(caller, query, response)
	default:
		return nil
	}
}

func responseProcessorGet(caller *Caller, query *Query, response *jsonrpc.RPCResponse) error {
	var result map[string]interface{}
	var receipt *PurchaseReceipt
	var urlSuffix string

	url := query.ParamsAsMap()["uri"].(string)
	log := logger.Log().WithField("url", url)

	basicGetResp := &ljsonrpc.GetResponse{}

	err := response.GetObject(&result)
	if err != nil {
		return err
	}

	err = ljsonrpc.Decode(result, basicGetResp)
	if err != nil {
		return err
	}

	// If this is a lbrynet error response, don't attempt to process it
	if basicGetResp.ClaimName == "" {
		return nil
	}

	stream := basicGetResp.Metadata.GetStream()
	if stream.Fee != nil && stream.Fee.Amount > 0 {
		if receipt = checkReceipt(result); receipt == nil {
			log.Debugf("receipt not found on a paid stream, trying to resolve")
			url := query.ParamsAsMap()["uri"].(string)
			resReq := &jsonrpc.RPCRequest{
				Method: MethodResolve,
				Params: map[string]string{
					"urls": url,
				},
				JSONRPC: "2.0",
			}

			fmt.Println(resReq)
			resQuery, err := NewQuery(resReq, query.WalletID)
			if err != nil {
				return err
			}
			resRespRaw, err := caller.callQueryWithRetry(resQuery)
			if err != nil {
				return err
			}

			log.Debugf("got resolve response")
			resResult := map[string]interface{}{}
			err = resRespRaw.GetObject(&resResult)
			if err != nil {
				log.Debug("error parsing resolve response:", err)
				return err
			}
			receipt = checkReceipt(resResult[url].(map[string]interface{}))
		}
	}

	if receipt != nil {
		log.Debug("found purchase receipt")
		token, err := paid.CreateToken(basicGetResp.ClaimName+"/"+basicGetResp.ClaimID, receipt.Txid, basicGetResp.Metadata.GetStream().Source.Size, paid.ExpTenSecPer100MB)
		if err != nil {
			return err
		}
		urlSuffix = fmt.Sprintf("paid/%s/%s/%s", basicGetResp.ClaimName, basicGetResp.ClaimID, token)
	} else {
		urlSuffix = fmt.Sprintf("free/%s/%s", basicGetResp.ClaimName, basicGetResp.ClaimID)
	}

	result["streaming_url"] = config.GetConfig().Viper.GetString("BaseContentURL") + urlSuffix
	result["purchase_receipt"] = receipt
	response.Result = result
	return nil
}

func checkReceipt(obj map[string]interface{}) *PurchaseReceipt {
	receipt := &PurchaseReceipt{}
	err := ljsonrpc.Decode(obj["purchase_receipt"], &receipt)
	if err != nil {
		logger.Log().Debug("error decoding receipt:", err)
		return nil
	}
	return receipt
}
