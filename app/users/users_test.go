package users

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/router"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/models"
	"github.com/lbryio/lbrytv/util/wallet"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
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
	return launchDummyAPIServer([]byte(
		fmt.Sprintf(`
		{
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

func launchEasyAPIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := r.PostFormValue("auth_token")

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		reply := fmt.Sprintf(`
		{
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
		}`, t)
		w.Write([]byte(reply))
	}))
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

	code := m.Run()

	os.Exit(code)
}

func setupDBTables() {
	storage.Conn.Truncate([]string{"users"})
}

func setupCleanupDummyUser(uidParam ...int) func() {
	var uid int
	if len(uidParam) > 0 {
		uid = uidParam[0]
	} else {
		uid = dummyUserID
	}

	ts := launchAuthenticatingAPIServer(uid)
	config.Override("InternalAPIHost", ts.URL)

	return func() {
		ts.Close()
		config.RestoreOverridden()
		lbrynet.WalletRemove(uid)
	}
}

func TestWalletServiceRetrieveNewUser(t *testing.T) {
	setupDBTables()
	defer setupCleanupDummyUser()()

	wid := wallet.MakeID(dummyUserID)
	svc := NewWalletService()
	u, err := svc.Retrieve(Query{Token: "abc"})
	require.NoError(t, err, errors.Unwrap(err))
	require.NotNil(t, u)
	require.Equal(t, wid, u.WalletID)

	count, err := models.Users(models.UserWhere.ID.EQ(u.ID)).CountG()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	u, err = svc.Retrieve(Query{Token: "abc"})
	require.NoError(t, err, errors.Unwrap(err))
	require.Equal(t, wid, u.WalletID)
}

func TestWalletServiceRetrieveNonexistentUser(t *testing.T) {
	setupDBTables()

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
	require.Error(t, err)
	require.Nil(t, u)
	assert.Equal(t, "cannot authenticate user with internal-apis: could not authenticate user", err.Error())
}

func TestWalletServiceRetrieveExistingUser(t *testing.T) {
	setupDBTables()
	defer setupCleanupDummyUser()()

	s := NewWalletService()
	u, err := s.Retrieve(Query{Token: "abc"})
	require.NoError(t, err)
	require.NotNil(t, u)

	u, err = s.Retrieve(Query{Token: "abc"})
	require.NoError(t, err)
	assert.EqualValues(t, dummyUserID, u.ID)

	count, err := models.Users().CountG()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestWalletServiceRetrieveExistingUserMissingWalletID(t *testing.T) {
	setupDBTables()

	uid := int(rand.Int31())
	ts := launchAuthenticatingAPIServer(uid)
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	s := NewWalletService()
	u, err := s.createDBUser(uid)
	require.NoError(t, err)
	require.NotNil(t, u)

	u, err = s.Retrieve(Query{Token: "abc"})
	require.NoError(t, err)
	assert.NotEqual(t, "", u.WalletID)
}

func TestWalletServiceRetrieveEmptyEmailNoUser(t *testing.T) {
	setupDBTables()

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
	assert.NoError(t, err)
}

func BenchmarkWalletCommands(b *testing.B) {
	setupDBTables()

	ts := launchEasyAPIServer()
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	walletsNum := 60
	users := make([]*models.User, walletsNum)
	svc := NewWalletService()
	sdkRouter := router.NewDefault()
	cl := jsonrpc.NewClient(sdkRouter.GetBalancedSDKAddress())

	svc.Logger.Disable()
	lbrynet.Logger.Disable()
	log.SetOutput(ioutil.Discard)

	rand.Seed(time.Now().UnixNano())

	for i := 0; i < walletsNum; i++ {
		uid := int(rand.Int31())
		u, err := svc.Retrieve(Query{Token: fmt.Sprintf("%v", uid)})
		require.NoError(b, err, errors.Unwrap(err))
		require.NotNil(b, u)
		users[i] = u
	}

	b.SetParallelism(20)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			u := users[rand.Intn(len(users))]
			res, err := cl.Call("account_balance", map[string]string{"wallet_id": u.WalletID})
			require.NoError(b, err)
			assert.Nil(b, res.Error)
		}
	})

	b.StopTimer()
}
