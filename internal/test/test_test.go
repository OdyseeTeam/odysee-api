package test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockHTTPServer(t *testing.T) {
	reqChan := ReqChan()
	rpcServer := MockHTTPServer(reqChan)
	defer rpcServer.Close()

	rpcServer.NextResponse <- `{"result": {"items": [], "page": 1, "page_size": 2, "total_pages": 3}}`
	res, err := ljsonrpc.NewClient(rpcServer.URL).WalletList("", 1, 2)
	require.NoError(t, err)

	req := <-reqChan
	assert.Equal(t, req.R.Method, http.MethodPost)
	assert.Equal(t, req.Body, `{"method":"wallet_list","params":{"page":1,"page_size":2},"id":0,"jsonrpc":"2.0"}`)

	assert.Equal(t, res.Page, uint64(1))
	assert.Equal(t, res.PageSize, uint64(2))
	assert.Equal(t, res.TotalPages, uint64(3))

	rpcServer.NextResponse <- `ok`
	c := &http.Client{}
	r, err := http.NewRequest(http.MethodPost, rpcServer.URL, bytes.NewBuffer([]byte("hello")))
	require.NoError(t, err)
	res2, err := c.Do(r)
	require.NoError(t, err)

	req2 := <-reqChan
	assert.Equal(t, req2.R.Method, http.MethodPost)
	assert.Equal(t, req2.Body, `hello`)
	body, err := ioutil.ReadAll(res2.Body)
	require.NoError(t, err)
	assert.Equal(t, string(body), "ok")
}

func TestAssertEqualJSON(t *testing.T) {
	testCases := []struct {
		a, b string
		same bool
	}{
		{"{}", "12", false},
		{"{}", "{}", true},
		{"{}", "", false},
		{`{"a":1,"b":2}`, `{"b":2,"a":1}`, true},
	}

	for i, tc := range testCases {
		testT := &testing.T{}
		same := AssertEqualJSON(testT, tc.a, tc.b)
		if tc.same {
			assert.True(t, same, "Case %d same", i)
			assert.False(t, testT.Failed(), "Case %d failure", i)
		} else {
			assert.False(t, same, "Case %d same", i)
			assert.True(t, testT.Failed(), "Case %d failure", i)
		}
	}
}
