package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/db"
	"github.com/lbryio/lbrytv/lbrynet"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

const dummyServerURL = "http://127.0.0.1:59999"
const proxySuffix = "/api/proxy"

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func launchDummyAPIServer(response []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(response)
	}))
}

func cleanup() {
	lbrynet.RemoveAccount("andrey@lbry.com")
	db.Cleanup(*db.Conn)
}

func TestWithValidAuthToken(t *testing.T) {
	cleanup()

	var (
		q        *jsonrpc.RPCRequest
		qBody    []byte
		response jsonrpc.RPCResponse
		account  ljsonrpc.Account
	)

	ts := launchDummyAPIServer([]byte(`{
		"success": true,
		"error": null,
		"data": {
		  "id": 751365,
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
	}`))
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	q = jsonrpc.NewRequest("account_list")
	qBody, _ = json.Marshal(q)
	r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))
	r.Header.Add("X-Lbry-Auth-Token", "d94ab9865f8416d107935d2ca644509c")

	rr := httptest.NewRecorder()
	http.HandlerFunc(Proxy).ServeHTTP(rr, r)
	require.Equal(t, http.StatusOK, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.Nil(t, err)
	require.Nil(t, response.Error)
	err = ljsonrpc.Decode(response.Result, &account)
	require.Nil(t, err)
	assert.Equal(t, lbrynet.AccountNameFromEmail("andrey@lbry.com"), account.Name)
}

func TestWithWrongAuthToken(t *testing.T) {
	cleanup()

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
	http.HandlerFunc(Proxy).ServeHTTP(rr, r)
	assert.Equal(t, http.StatusForbidden, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.Nil(t, err)
	assert.Equal(t, "could not authenticate user", response.Error.Message)
}

func TestWithoutToken(t *testing.T) {
	cleanup()
	// Create a dummy account so we have a wallet beside the default one
	lbrynet.CreateAccount("dummy@email.com")

	var (
		q        *jsonrpc.RPCRequest
		qBody    []byte
		response jsonrpc.RPCResponse
		account  ljsonrpc.Account
	)

	q = jsonrpc.NewRequest("account_list")
	qBody, _ = json.Marshal(q)
	r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))

	rr := httptest.NewRecorder()
	http.HandlerFunc(Proxy).ServeHTTP(rr, r)
	require.Equal(t, http.StatusOK, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &response)

	require.Nil(t, err)
	require.Nil(t, response.Error)
	err = ljsonrpc.Decode(response.Result, &account)
	require.Nil(t, err)
	assert.True(t, account.IsDefault, account)
}
