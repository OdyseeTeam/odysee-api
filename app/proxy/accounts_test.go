package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/lbrynet"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestWithValidAuthTokenConcurrent(t *testing.T) {
	t.Skip()
	// This test requires its own dummy account ID
	lbrynet.RemoveAccount(123123)
	defer lbrynet.RemoveAccount(123123)

	http.DefaultClient.Timeout = 10 * time.Second

	var wg sync.WaitGroup

	ts := launchDummyAPIServerDelayed([]byte(`{
		"success": true,
		"error": null,
		"data": {
		  "id": 123123,
		  "language": "en",
		  "given_name": null,
		  "family_name": null,
		  "created_at": "2019-01-17T12:13:06Z",
		  "updated_at": "2019-05-02T13:57:59Z",
		  "invited_by_id": null,
		  "invited_at": null,
		  "invites_remaining": 0,
		  "invite_reward_claimed": false,
		  "is_email_enabled": true,
		  "manual_approval_user_id": 837139,
		  "reward_status_change_trigger": "manual",
		  "primary_email": "andrey@lbry.com",
		  "has_verified_email": true,
		  "is_identity_verified": false,
		  "is_reward_approved": true,
		  "groups": []
		}
	}`), 500)
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	for w := range [10]int{} {
		wg.Add(1)
		go func(w int, wg *sync.WaitGroup) {
			var response jsonrpc.RPCResponse
			q := jsonrpc.NewRequest("account_list")

			qBody, _ := json.Marshal(q)
			r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))
			r.Header.Add("X-Lbry-Auth-Token", "d94ab9865f8416d107935d2ca644509c")

			rr := httptest.NewRecorder()
			handler := NewRequestHandler(svc)
			handler.Handle(rr, r)

			require.Equal(t, http.StatusOK, rr.Code)
			json.Unmarshal(rr.Body.Bytes(), &response)
			require.Nil(t, response.Error)
			wg.Done()
		}(w, &wg)
	}
	wg.Wait()
}

func TestWithWrongAuthToken(t *testing.T) {
	testFuncSetup()
	defer testFuncTeardown()
	config.Override("AccountsEnabled", true)
	var (
		q        *jsonrpc.RPCRequest
		qBody    []byte
		response jsonrpc.RPCResponse
	)

	ts := launchDummyAPIServer([]byte(`{
		"success": false,
		"error": "could not authenticate user",
		"data": null
	}`))
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	q = jsonrpc.NewRequest("account_list")
	qBody, _ = json.Marshal(q)
	r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))
	r.Header.Add("X-Lbry-Auth-Token", "xXxXxXx")

	rr := httptest.NewRecorder()
	handler := NewRequestHandler(svc)
	handler.Handle(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.Nil(t, err)
	assert.Equal(t, "cannot authenticate user with internal-apis: could not authenticate user", response.Error.Message)
}

func TestWithoutToken(t *testing.T) {
	testFuncSetup()
	defer testFuncTeardown()

	var (
		q              *jsonrpc.RPCRequest
		qBody          []byte
		response       jsonrpc.RPCResponse
		statusResponse ljsonrpc.StatusResponse
	)

	q = jsonrpc.NewRequest("status")
	qBody, _ = json.Marshal(q)
	r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))

	rr := httptest.NewRecorder()
	handler := NewRequestHandler(svc)
	handler.Handle(rr, r)
	require.Equal(t, http.StatusOK, rr.Code)

	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.Nil(t, err)
	require.Nil(t, response.Error)

	err = ljsonrpc.Decode(response.Result, &statusResponse)
	require.Nil(t, err)
	assert.True(t, statusResponse.IsRunning)
}

func TestAccountSpecificWithoutToken(t *testing.T) {
	testFuncSetup()
	defer testFuncTeardown()

	var (
		q        *jsonrpc.RPCRequest
		qBody    []byte
		response jsonrpc.RPCResponse
	)

	q = jsonrpc.NewRequest("account_list")
	qBody, _ = json.Marshal(q)
	r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))

	rr := httptest.NewRecorder()
	handler := NewRequestHandler(svc)
	handler.Handle(rr, r)
	require.Equal(t, http.StatusOK, rr.Code)

	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.Nil(t, err)
	require.NotNil(t, response.Error)
	require.Equal(t, "account identificator required", response.Error.Message)
}
