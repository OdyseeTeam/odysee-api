package query

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/lbryio/lbrytv-player/pkg/paid"
	"github.com/lbryio/lbrytv/config"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/ybbus/jsonrpc"
)

var reAlreadyPurchased = regexp.MustCompile(`(?i)You already have a purchase for claim_id`)

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
		res := responseProcessorGet(caller, query, response)
		return res
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
	isPaidStream := stream.Fee != nil && stream.Fee.Amount > 0
	if isPaidStream {
		purchaseQuery, err := NewQuery(jsonrpc.NewRequest(
			MethodPurchaseCreate,
			map[string]interface{}{
				"url":      url,
				"blocking": true,
			},
		), query.WalletID)
		if err != nil {
			return err
		}
		purchaseRes, err := caller.callQueryWithRetry(purchaseQuery)
		if err != nil {
			return err
		}
		if purchaseRes.Error != nil && !reAlreadyPurchased.MatchString(purchaseRes.Error.Message) {
			return fmt.Errorf("purchase error: %v", purchaseRes.Error.Message)
		}

		url := query.ParamsAsMap()["uri"].(string)

		resQuery, err := NewQuery(jsonrpc.NewRequest(
			MethodResolve,
			map[string]interface{}{
				"urls":                     url,
				"include_purchase_receipt": true,
			},
		), query.WalletID)
		if err != nil {
			return err
		}

		log.Debug("receipt not found on a paid stream, trying to resolve")
		resRespRaw, err := caller.callQueryWithRetry(resQuery)
		if err != nil {
			return err
		}

		resResult := map[string]interface{}{}
		err = resRespRaw.GetObject(&resResult)
		if err != nil {
			log.Debug("error parsing resolve response:", err)
			return err
		}
		if resEntry, ok := resResult[url]; ok {
			receipt = checkReceipt(resEntry.(map[string]interface{}))
			if receipt != nil {
				log.Debugf("found receipt in resolve")
			}
		} else {
			log.Debug("couldn't retrieve resolve response entry")
		}
	}

	if receipt != nil {
		log.Debug("found purchase receipt")
		token, err := paid.CreateToken(basicGetResp.ClaimName+"/"+basicGetResp.ClaimID, receipt.Txid, basicGetResp.Metadata.GetStream().Source.Size, paid.ExpTenSecPer100MB)
		if err != nil {
			return err
		}
		urlSuffix = fmt.Sprintf("paid/%s/%s/%s", basicGetResp.ClaimName, basicGetResp.ClaimID, token)
		result["purchase_receipt"] = receipt
	} else if !isPaidStream {
		urlSuffix = fmt.Sprintf("free/%s/%s", basicGetResp.ClaimName, basicGetResp.ClaimID)
	} else {
		log.Error("purchase receipt missing")
		return errors.New("purchase receipt missing on a paid stream")
	}

	result["streaming_url"] = config.GetConfig().Viper.GetString("BaseContentURL") + urlSuffix

	response.Result = result
	return nil
}

func checkReceipt(obj map[string]interface{}) *PurchaseReceipt {
	var receipt *PurchaseReceipt
	err := ljsonrpc.Decode(obj["purchase_receipt"], &receipt)
	if err != nil {
		logger.Log().Debug("error decoding receipt:", err)
		return nil
	}
	return receipt
}
