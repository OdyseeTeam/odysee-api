package users

import (
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

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func cleanup() {
	lbrynet.RemoveAccount(dummyUserID)
	storage.Cleanup(*storage.Conn, []string{
		"users",
	})
}

func TestGetUser_New(t *testing.T) {
	cleanup()

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

	u, err := NewUserService("abc").GetUser()
	require.Nil(t, err)
	require.NotNil(t, u)

	count, err := models.Users(models.UserWhere.ID.EQ(u.ID)).CountG()
	require.Nil(t, err)
	assert.EqualValues(t, 1, count)
}

func TestGetUser_Existing(t *testing.T) {
	cleanup()

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

	s := NewUserService("abc")

	u, err := s.GetUser()
	require.Nil(t, err)
	require.NotNil(t, u)

	u, err = s.GetUser()
	require.Nil(t, err)
	assert.EqualValues(t, dummyUserID, u.ID)

	count, err := models.Users().CountG()
	require.Nil(t, err)
	assert.EqualValues(t, 1, count)
}

func TestGetUser_Nonexistent(t *testing.T) {
	cleanup()
	ts := launchDummyAPIServer([]byte(`{
		"success": false,
		"error": "could not authenticate user",
		"data": null
	}`))
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	u, err := NewUserService("non-existent-token").GetUser()
	require.NotNil(t, err)
	require.Nil(t, u)
	assert.Equal(t, "cannot authenticate user with internal-apis: could not authenticate user", err.Error())
}

func TestGetUser_EmptyEmail_NoUser(t *testing.T) {
	cleanup()
	ts := launchDummyAPIServer([]byte(`{
		"success": true,
		"error": null,
		"data": {
			"id": 1000985,
			"language": "en",
			"given_name": null,
			"family_name": null,
			"created_at": "2019-05-30T13:24:57Z",
			"updated_at": "2019-05-30T13:31:07Z",
			"invited_by_id": 756576,
			"invited_at": null,
			"invites_remaining": 0,
			"invite_reward_claimed": false,
			"is_email_enabled": true,
			"manual_approval_user_id": null,
			"reward_status_change_trigger": null,
			"primary_email": null,
			"has_verified_email": false,
			"is_identity_verified": false,
			"is_reward_approved": false,
			"groups": []
		}
	}`))
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	u, err := NewUserService("abc").GetUser()
	assert.Nil(t, u)
	assert.EqualError(t, err, "cannot authenticate user: email is empty/not confirmed")
}

func TestGetAccountIDFromRequest_NoToken(t *testing.T) {
	r, _ := http.NewRequest("POST", "/", nil)

	id, err := GetAccountIDFromRequest(r)
	require.Nil(t, err)
	assert.Equal(t, "", id)
}

func TestGetAccountIDFromRequest_Existing(t *testing.T) {
	cleanup()

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

	r, _ := http.NewRequest("POST", "/", nil)
	r.Header.Add(TokenHeader, "abc")

	id, err := GetAccountIDFromRequest(r)
	require.Nil(t, err)

	u, err := NewUserService("abc").GetUser()
	require.Nil(t, err)

	assert.EqualValues(t, u.SDKAccountID, id)
}

func TestGetAccountIDFromRequest_Nonexistent(t *testing.T) {
	cleanup()
	ts := launchDummyAPIServer([]byte(`{
		"success": false,
		"error": "could not authenticate user",
		"data": null
	}`))
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	r, _ := http.NewRequest("POST", "/", nil)
	r.Header.Add(TokenHeader, "abc")

	id, err := GetAccountIDFromRequest(r)
	require.NotNil(t, err)
	assert.Equal(t, "cannot authenticate user with internal-apis: could not authenticate user", err.Error())
	assert.Equal(t, "", id)
}
