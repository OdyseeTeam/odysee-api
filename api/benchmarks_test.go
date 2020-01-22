package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/router"
	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/responses"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

const proxySuffix = "/api/v1/proxy"

func launchAuthenticatingAPIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := r.PostFormValue("auth_token")

		responses.PrepareJSONWriter(w)

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

func BenchmarkWalletCommands(b *testing.B) {
	setupDBTables()

	ts := launchAuthenticatingAPIServer()
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	walletsNum := 30
	wallets := make([]*models.User, walletsNum)
	svc := users.NewWalletService()

	svc.Logger.Disable()
	lbrynet.Logger.Disable()
	log.SetOutput(ioutil.Discard)

	rand.Seed(time.Now().UnixNano())

	for i := 0; i < walletsNum; i++ {
		uid := int(rand.Int31())
		u, err := svc.Retrieve(users.Query{Token: fmt.Sprintf("%v", uid)})
		require.NoError(b, err, errors.Unwrap(err))
		require.NotNil(b, u)
		wallets[i] = u
	}

	handler := proxy.NewRequestHandler(
		proxy.NewService(
			proxy.Opts{SDKRouter: router.New(config.GetLbrynetServers())},
		),
	)

	b.SetParallelism(30)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			u := wallets[rand.Intn(len(wallets))]

			var response jsonrpc.RPCResponse
			q := jsonrpc.NewRequest("wallet_balance", map[string]string{"wallet_id": u.WalletID})

			qBody, _ := json.Marshal(q)
			r, _ := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))
			r.Header.Add("X-Lbry-Auth-Token", fmt.Sprintf("%v", u.ID))

			rr := httptest.NewRecorder()
			handler.Handle(rr, r)

			require.Equal(b, http.StatusOK, rr.Code)
			json.Unmarshal(rr.Body.Bytes(), &response)
			require.Nil(b, response.Error)
		}
	})

	b.StopTimer()
}
