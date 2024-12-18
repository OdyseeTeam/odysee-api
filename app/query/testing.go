package query

import (
	"encoding/json"
	"strings"

	"github.com/ybbus/jsonrpc/v2"
)

func decodeResponse(r string) (*jsonrpc.RPCResponse, error) {
	decoder := json.NewDecoder(strings.NewReader(r))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	response := &jsonrpc.RPCResponse{}
	return response, decoder.Decode(response)
}
