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
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/config"
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

		responses.AddJSONContentType(w)

		reply := fmt.Sprintf(`
		{
			"success": true,
			"error": null,
			"data": {
				"id": %s,
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

	wallet.DisableLogger()
	sdkrouter.DisableLogger()
	log.SetOutput(ioutil.Discard)

	rand.Seed(time.Now().UnixNano())

	rt := sdkrouter.New(config.GetLbrynetServers())

	ts := launchAuthenticatingAPIServer()
	defer ts.Close()

	walletsNum := 30
	wallets := make([]*models.User, walletsNum)

	for i := 0; i < walletsNum; i++ {
		uid := int(rand.Int31())
		u, err := wallet.GetUserWithWallet(rt, ts.URL, fmt.Sprintf("%d", uid), "")
		require.NoError(b, err, errors.Unwrap(err))
		require.NotNil(b, u)
		wallets[i] = u
	}

	handler := sdkrouter.Middleware(rt)(http.HandlerFunc(proxy.Handle))

	b.SetParallelism(30)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			u := wallets[rand.Intn(len(wallets))]

			q := jsonrpc.NewRequest("wallet_balance", map[string]string{"wallet_id": u.WalletID})

			qBody, err := json.Marshal(q)
			require.NoError(b, err)
			r, err := http.NewRequest("POST", proxySuffix, bytes.NewBuffer(qBody))
			require.NoError(b, err)
			r.Header.Add("X-Lbry-Auth-Token", fmt.Sprintf("%d", u.ID))

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, r)

			require.Equal(b, http.StatusOK, rr.Code)

			var response jsonrpc.RPCResponse
			json.Unmarshal(rr.Body.Bytes(), &response)
			require.Nil(b, response.Error)
		}
	})

	b.StopTimer()
}
