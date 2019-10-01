package users

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletServiceRetrieveNewUser(t *testing.T) {
	testFuncSetup()

	ts := launchAuthenticatingAPIServer(dummyUserID)
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	wid := lbrynet.MakeWalletID(dummyUserID)
	svc := NewWalletService()
	u, err := svc.Retrieve(Query{Token: "abc"})
	require.Nil(t, err, errors.Unwrap(err))
	require.NotNil(t, u)
	require.Equal(t, wid, u.WalletID)

	count, err := models.Users(models.UserWhere.ID.EQ(u.ID)).CountG()
	require.Nil(t, err)
	assert.EqualValues(t, 1, count)

	u, err = svc.Retrieve(Query{Token: "abc"})
	require.Nil(t, err, errors.Unwrap(err))
	require.Equal(t, wid, u.WalletID)
}

func TestWalletServiceRetrieveNonexistentUser(t *testing.T) {
	testFuncSetup()

	ts := launchDummyAPIServer([]byte(`{
		"success": false,
		"error": "could not authenticate user",
		"data": null
	}`))
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	svc := NewWalletService()
	u, err := svc.Retrieve(Query{Token: "non-existent-token"})
	require.NotNil(t, err)
	require.Nil(t, u)
	assert.Equal(t, "cannot authenticate user with internal-apis: could not authenticate user", err.Error())
}

func TestWalletServiceRetrieveExistingUser(t *testing.T) {
	testFuncSetup()

	ts := launchAuthenticatingAPIServer(dummyUserID)
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	s := NewWalletService()
	u, err := s.Retrieve(Query{Token: "abc"})
	require.Nil(t, err)
	require.NotNil(t, u)

	u, err = s.Retrieve(Query{Token: "abc"})
	require.Nil(t, err)
	assert.EqualValues(t, dummyUserID, u.ID)

	count, err := models.Users().CountG()
	require.Nil(t, err)
	assert.EqualValues(t, 1, count)
}

func TestWalletServiceRetrieveExistingUserMissingWalletID(t *testing.T) {
	testFuncSetup()

	uid := int(rand.Int31())
	ts := launchAuthenticatingAPIServer(uid)
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	s := NewWalletService()
	u, err := s.createDBUser(uid)
	require.Nil(t, err)
	require.NotNil(t, u)

	u, err = s.Retrieve(Query{Token: "abc"})
	require.Nil(t, err)
	assert.NotEqual(t, "", u.WalletID)
}

func TestWalletServiceRetrieveEmptyEmailNoUser(t *testing.T) {
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

	svc := NewWalletService()
	u, err := svc.Retrieve(Query{Token: "abc"})
	assert.Nil(t, u)
	assert.EqualError(t, err, "cannot authenticate user with internal-api, email not confirmed")
}
