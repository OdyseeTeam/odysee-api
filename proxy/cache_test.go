package proxy

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ybbus/jsonrpc"
)

func TestCache(t *testing.T) {
	var (
		response jsonrpc.RPCResponse
	)
	responseCache.flush()
	params := map[string]interface{}{"urls": []string{"one", "two", "three"}}

	absPath, _ := filepath.Abs("./testdata/resolve_response.json")
	rawJSON, err := ioutil.ReadFile(absPath)
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(rawJSON, &response)
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, responseCache.Retrieve("resolve", params))
	responseCache.Save("resolve", params, response.Result)
	assert.Equal(t, 1, responseCache.Count())
	assert.Equal(t, response.Result, responseCache.Retrieve("resolve", params))
}
