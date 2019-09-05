package users

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dummyUserID = 751365
const dummyServerURL = "http://127.0.0.1:59988"

func launchDummyAPIServer(response []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(response)
	}))
}

func launchAuthenticatingAPIServer(userID int) *httptest.Server {
	return launchDummyAPIServer([]byte(fmt.Sprintf(`{
		"success": true,
		"error": null,
		"data": {
		  "id": %v,
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
		  "primary_email": "user@domain.com",
		  "has_verified_email": true,
		  "is_identity_verified": false,
		  "is_reward_approved": true,
		  "groups": []
		}
	}`, userID)))
}

func TestMain(m *testing.M) {
	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}
	dbConn, connCleanup := storage.CreateTestConn(params)
	dbConn.SetDefaultConnection()
	defer connCleanup()
	defer lbrynet.RemoveAccount(dummyUserID)

	code := m.Run()

	os.Exit(code)
}

func testFuncSetup() {
	lbrynet.RemoveAccount(dummyUserID)
	storage.Conn.Truncate([]string{"users"})
}

func TestRetrieve_New(t *testing.T) {
	testFuncSetup()

	ts := launchAuthenticatingAPIServer(dummyUserID)
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	u, err := NewUserService().Retrieve("abc")
	require.Nil(t, err)
	require.NotNil(t, u)

	count, err := models.Users(models.UserWhere.ID.EQ(u.ID)).CountG()
	require.Nil(t, err)
	assert.EqualValues(t, 1, count)
}

func TestRetrieve_Existing(t *testing.T) {
	testFuncSetup()

	ts := launchAuthenticatingAPIServer(dummyUserID)
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	s := NewUserService()

	u, err := s.Retrieve("abc")
	require.Nil(t, err)
	require.NotNil(t, u)

	u, err = s.Retrieve("abc")
	require.Nil(t, err)
	assert.EqualValues(t, dummyUserID, u.ID)

	count, err := models.Users().CountG()
	require.Nil(t, err)
	assert.EqualValues(t, 1, count)
}

func TestRetrieve_Nonexistent(t *testing.T) {
	testFuncSetup()

	ts := launchDummyAPIServer([]byte(`{
		"success": false,
		"error": "could not authenticate user",
		"data": null
	}`))
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	u, err := NewUserService().Retrieve("non-existent-token")
	require.NotNil(t, err)
	require.Nil(t, u)
	assert.Equal(t, "cannot authenticate user with internal-apis: could not authenticate user", err.Error())
}

func TestRetrieve_EmptyEmail_NoUser(t *testing.T) {
	testFuncSetup()

	// API server returns empty email
	ts := launchDummyAPIServer([]byte(`{
		"success": true,
		"error": null,
		"data": {
		  "id": 111111111,
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
		  "primary_email": null,
		  "has_verified_email": true,
		  "is_identity_verified": false,
		  "is_reward_approved": true,
		  "groups": []
		}
	}`))
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	u, err := NewUserService().Retrieve("abc")
	assert.Nil(t, u)
	assert.EqualError(t, err, "cannot authenticate user: email not confirmed")
}

func TestGetAccountIDFromRequestNoToken(t *testing.T) {
	r, _ := http.NewRequest("POST", "/", nil)

	svc := NewUserService()
	id, err := GetAccountIDFromRequest(r, svc)
	require.Nil(t, err)
	assert.Equal(t, "", id)
}

func TestGetAccountIDFromRequestExisting(t *testing.T) {
	testFuncSetup()

	ts := launchAuthenticatingAPIServer(dummyUserID)
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	r, _ := http.NewRequest("POST", "/", nil)
	r.Header.Add(TokenHeader, "abc")

	svc := NewUserService()
	id, err := GetAccountIDFromRequest(r, svc)
	require.Nil(t, err)

	u, err := svc.Retrieve("abc")
	require.Nil(t, err)

	assert.EqualValues(t, u.SDKAccountID, id)
}

func TestGetAccountIDFromRequestNonexistent(t *testing.T) {
	testFuncSetup()

	ts := launchDummyAPIServer([]byte(`{
		"success": false,
		"error": "could not authenticate user",
		"data": null
	}`))
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()
	svc := NewUserService()

	r, _ := http.NewRequest("POST", "/", nil)
	r.Header.Add(TokenHeader, "abc")

	id, err := GetAccountIDFromRequest(r, svc)
	require.NotNil(t, err)
	assert.Equal(t, "cannot authenticate user with internal-apis: could not authenticate user", err.Error())
	assert.Equal(t, "", id)
}

func TestCreateDBUserForExistingSDKAccount(t *testing.T) {
	testFuncSetup()

	ts := launchAuthenticatingAPIServer(dummyUserID)
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	acc, err := lbrynet.CreateAccount(dummyUserID)
	require.Nil(t, err)

	r, _ := http.NewRequest("POST", "/", nil)
	r.Header.Add(TokenHeader, "abc")

	svc := NewUserService()
	id, err := GetAccountIDFromRequest(r, svc)
	require.Nil(t, err)

	u, err := svc.Retrieve("abc")
	require.Nil(t, err)

	assert.EqualValues(t, u.SDKAccountID, acc.ID)
	assert.EqualValues(t, u.SDKAccountID, id)

	uRetrieved, err := svc.getDBUser(u.ID)
	require.Nil(t, err)

	assert.EqualValues(t, uRetrieved.SDKAccountID, acc.ID)
}
