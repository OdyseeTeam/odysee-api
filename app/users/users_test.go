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

func llll() *httptest.Server {
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
	defer lbrynet.RemoveAccount(dummyUserID)

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
