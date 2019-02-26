package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/lbryio/lbry.go/extras/errors"
	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
	lbryschema "github.com/lbryio/types/go"
	"github.com/mitchellh/mapstructure"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/ybbus/jsonrpc"
)

func TestProxyNilQuery(t *testing.T) {
	request, _ := http.NewRequest("POST", "/api/proxy", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(Proxy).ServeHTTP(rr, request)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "empty request body", rr.Body.String())
}

func TestProxyNonsenseQuery(t *testing.T) {
	var parsedResponse jsonrpc.RPCResponse

	request, _ := http.NewRequest("POST", "/api/proxy", bytes.NewBuffer([]byte("yo")))
	rr := httptest.NewRecorder()
	http.HandlerFunc(Proxy).ServeHTTP(rr, request)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	if err != nil {
		panic(err)
	}
	assert.True(t, strings.HasPrefix(parsedResponse.Error.Message, "client json parse error: invalid character 'y'"))
}

func decode(data interface{}, targetStruct interface{}) error {
	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   targetStruct,
		TagName:  "json",
		//WeaklyTypedInput: true,
		DecodeHook: fixDecodeProto,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		panic(err)
	}

	err = decoder.Decode(data)
	if err != nil {
		panic(err)
	}
	return nil
}

func fixDecodeProto(src, dest reflect.Type, data interface{}) (interface{}, error) {
	switch dest {
	case reflect.TypeOf(uint64(0)):
		if n, ok := data.(json.Number); ok {
			val, err := n.Int64()
			if err != nil {
				return nil, errors.Wrap(err, 0)
			} else if val < 0 {
				return nil, errors.Err("must be unsigned int")
			}
			return uint64(val), nil
		}
	case reflect.TypeOf([]byte{}):
		if s, ok := data.(string); ok {
			return []byte(s), nil
		}

	case reflect.TypeOf(decimal.Decimal{}):
		if n, ok := data.(json.Number); ok {
			val, err := n.Float64()
			if err != nil {
				return nil, errors.Wrap(err, 0)
			}
			return decimal.NewFromFloat(val), nil
		} else if s, ok := data.(string); ok {
			d, err := decimal.NewFromString(s)
			if err != nil {
				return nil, errors.Wrap(err, 0)
			}
			return d, nil
		}

	case reflect.TypeOf(lbryschema.Metadata_Version(0)):
		val, err := getEnumVal(lbryschema.Metadata_Version_value, data)
		return lbryschema.Metadata_Version(val), err
	case reflect.TypeOf(lbryschema.Metadata_Language(0)):
		val, err := getEnumVal(lbryschema.Metadata_Language_value, data)
		return lbryschema.Metadata_Language(val), err

	case reflect.TypeOf(lbryschema.Stream_Version(0)):
		val, err := getEnumVal(lbryschema.Stream_Version_value, data)
		return lbryschema.Stream_Version(val), err

	case reflect.TypeOf(lbryschema.Claim_Version(0)):
		val, err := getEnumVal(lbryschema.Claim_Version_value, data)
		return lbryschema.Claim_Version(val), err
	case reflect.TypeOf(lbryschema.Claim_ClaimType(0)):
		val, err := getEnumVal(lbryschema.Claim_ClaimType_value, data)
		return lbryschema.Claim_ClaimType(val), err

	case reflect.TypeOf(lbryschema.Fee_Version(0)):
		val, err := getEnumVal(lbryschema.Fee_Version_value, data)
		return lbryschema.Fee_Version(val), err
	case reflect.TypeOf(lbryschema.Fee_Currency(0)):
		val, err := getEnumVal(lbryschema.Fee_Currency_value, data)
		return lbryschema.Fee_Currency(val), err

	case reflect.TypeOf(lbryschema.Source_Version(0)):
		val, err := getEnumVal(lbryschema.Source_Version_value, data)
		return lbryschema.Source_Version(val), err
	case reflect.TypeOf(lbryschema.Source_SourceTypes(0)):
		val, err := getEnumVal(lbryschema.Source_SourceTypes_value, data)
		return lbryschema.Source_SourceTypes(val), err

	case reflect.TypeOf(lbryschema.KeyType(0)):
		val, err := getEnumVal(lbryschema.KeyType_value, data)
		return lbryschema.KeyType(val), err

	case reflect.TypeOf(lbryschema.Signature_Version(0)):
		val, err := getEnumVal(lbryschema.Signature_Version_value, data)
		return lbryschema.Signature_Version(val), err

	case reflect.TypeOf(lbryschema.Certificate_Version(0)):
		val, err := getEnumVal(lbryschema.Certificate_Version_value, data)
		return lbryschema.Certificate_Version(val), err
	}

	return data, nil
}

func getEnumVal(enum map[string]int32, data interface{}) (int32, error) {
	s, ok := data.(string)
	if !ok {
		return 0, errors.Err("expected a string")
	}
	val, ok := enum[s]
	if !ok {
		return 0, errors.Err("invalid enum key")
	}
	return val, nil
}

func TestProxy(t *testing.T) {
	var query *jsonrpc.RPCRequest
	var queryBody []byte
	var parsedResponse jsonrpc.RPCResponse
	resolveResponse := make(ljsonrpc.ResolveResponse)

	query = jsonrpc.NewRequest("resolve", map[string]string{"urls": "what"})
	queryBody, _ = json.Marshal(query)
	request, _ := http.NewRequest("POST", "/api/proxy", bytes.NewBuffer(queryBody))
	rr := httptest.NewRecorder()

	http.HandlerFunc(Proxy).ServeHTTP(rr, request)

	assert.Equal(t, http.StatusOK, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	if err != nil {
		panic(err)
	}
	decode(parsedResponse.Result, &resolveResponse)
	assert.Equal(t, "what", resolveResponse["what"].Claim.Name)
}

func TestIndex(t *testing.T) {
	request, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	http.HandlerFunc(Index).ServeHTTP(rr, request)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `<script src="/static/app/bundle.js"></script>`)
}
