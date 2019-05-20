package routes

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/lbrynet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

const dummyServerURL = "http://127.0.0.1:59999"
const proxySuffix = "/api/proxy"

func launchDummyServer(response []byte) {
	s := &http.Server{
		Addr: "127.0.0.1:59999",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write(response)
		}),
	}
	log.Fatal(s.ListenAndServe())
}

func TestWithValidAuthToken(t *testing.T) {
	var (
		q              *jsonrpc.RPCRequest
		qBody          []byte
		parsedResponse jsonrpc.RPCResponse
		sdkResponse    lbrynet.SingleAccountListResponse
	)

	go launchDummyServer([]byte(`{
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
	config.Override("InternalAPIHost", dummyServerURL)
	defer config.RestoreOverridden()

	q = jsonrpc.NewRequest("accounts_list")
	qBody, _ = json.Marshal(q)
	r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))
	r.Header.Add("X-Lbry-Auth-Token", "d94ab9865f8416d107935d2ca644509c")

	rr := httptest.NewRecorder()
	http.HandlerFunc(Proxy).ServeHTTP(rr, r)
	require.Equal(t, http.StatusOK, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	if err != nil {
		t.Fatal(err)
	}
	ljsonrpc.Decode(parsedResponse.Result, &sdkResponse)
	assert.Equal(t, "andrey@lbry.com", sdkResponse.Name)
}

func TestWithWrongAuthToken(t *testing.T) {
	var (
		q              *jsonrpc.RPCRequest
		qBody          []byte
		parsedResponse jsonrpc.RPCResponse
	)

	go launchDummyServer([]byte(`{
		"success": false,
		"error": "could not authenticate user",
		"data": null
	}`))
	config.Override("InternalAPIHost", dummyServerURL)
	defer config.RestoreOverridden()

	q = jsonrpc.NewRequest("accounts_list")
	qBody, _ = json.Marshal(q)
	r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))
	r.Header.Add("X-Lbry-Auth-Token", "xXXx")

	rr := httptest.NewRecorder()
	http.HandlerFunc(Proxy).ServeHTTP(rr, r)
	require.Equal(t, http.StatusOK, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "Invalid auth_token", parsedResponse.Error.Message)
}

func TestWithoutToken(t *testing.T) {
	var (
		q              *jsonrpc.RPCRequest
		qBody          []byte
		parsedResponse jsonrpc.RPCResponse
		sdkResponse    ljsonrpc.AccountListResponse
	)

	q = jsonrpc.NewRequest("accounts_list")
	qBody, _ = json.Marshal(q)
	r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))
	rr := httptest.NewRecorder()
	http.HandlerFunc(Proxy).ServeHTTP(rr, r)

	require.Equal(t, http.StatusOK, rr.Code)
	err := json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	if err != nil {
		t.Fatal(err)
	}
	ljsonrpc.Decode(parsedResponse.Result, &sdkResponse)
	firstRequestID := sdkResponse.LBCMainnet[0].ID

	rr = httptest.NewRecorder()
	http.HandlerFunc(Proxy).ServeHTTP(rr, r)
	require.Equal(t, http.StatusOK, rr.Code)
	err = json.Unmarshal(rr.Body.Bytes(), &parsedResponse)
	if err != nil {
		t.Fatal(err)
	}
	ljsonrpc.Decode(parsedResponse.Result, &sdkResponse)
	assert.Equal(t, firstRequestID, sdkResponse.LBCMainnet[0].ID)
}
