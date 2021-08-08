package tracker

import (
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/internal/test"
	"github.com/lbryio/lbrytv/models"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/sirupsen/logrus"
	logrusTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
)

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())

	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options + "&TimeZone=UTC",
	}
	dbConn, connCleanup := storage.CreateTestConn(params)
	dbConn.SetDefaultConnection()

	code := m.Run()

	connCleanup()
	os.Exit(code)
}

func TestTouch(t *testing.T) {
	storage.Conn.Truncate([]string{models.TableNames.Users, models.TableNames.LbrynetServers})
	// create user
	u := &models.User{ID: rand.Intn(99999)}
	err := u.InsertG(boil.Infer())
	require.NoError(t, err)

	// reload, check that access time is null
	err = u.ReloadG()
	require.NoError(t, err)
	assert.False(t, u.LastSeenAt.Valid)

	// set access time back in the past
	oneHourAgo := TimeNow().Add(-1 * time.Hour)
	u.LastSeenAt = null.TimeFrom(oneHourAgo)
	_, err = u.UpdateG(boil.Infer())
	require.NoError(t, err)

	// reload, check that access time is not null
	err = u.ReloadG()
	require.NoError(t, err)
	assert.True(t, u.LastSeenAt.Valid)

	// touch user
	Touch(boil.GetDB(), u.ID)

	// reload, check that access time has been updated
	err = u.ReloadG()
	require.NoError(t, err)
	assert.True(t, u.LastSeenAt.Valid)
	assert.True(t, oneHourAgo.Before(u.LastSeenAt.Time))
}

func TestUnload(t *testing.T) {
	storage.Conn.Truncate([]string{models.TableNames.Users, models.TableNames.LbrynetServers})
	reqChan := test.ReqChan()
	ts := test.MockHTTPServer(reqChan)
	defer ts.Close()
	ts.NextResponse <- `{"id":1}` // for the wallet.Unload call

	// create models
	l := &models.LbrynetServer{ID: rand.Intn(99999), Name: "x", Address: ts.URL}
	err := l.InsertG(boil.Infer())
	require.NoError(t, err)

	u := &models.User{
		ID:              rand.Intn(99999),
		LbrynetServerID: null.IntFrom(l.ID),
		LastSeenAt:      null.TimeFrom(TimeNow().Add(-5 * time.Minute)),
	}
	err = u.InsertG(boil.Infer())
	require.NoError(t, err)

	// make sure we don't unload if wallet was used more recently than cutoff
	count, err := Unload(boil.GetDB(), 10*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// but we do if wallet is used less recently than cutoff
	count, err = Unload(boil.GetDB(), 1*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	req := <-reqChan
	assert.NotEmpty(t, req.Body)
}

func TestUnload_E2E(t *testing.T) {
	storage.Conn.Truncate([]string{models.TableNames.Users, models.TableNames.LbrynetServers})
	address := test.RandServerAddress(t)
	// create models
	l := &models.LbrynetServer{ID: rand.Intn(99999), Name: "xyz", Address: address}
	err := l.InsertG(boil.Infer())
	require.NoError(t, err)

	u := &models.User{
		ID:              rand.Intn(99999),
		LbrynetServerID: null.IntFrom(l.ID),
		LastSeenAt:      null.TimeFrom(TimeNow().Add(-10 * time.Minute)),
	}
	err = u.InsertG(boil.Infer())
	require.NoError(t, err)

	// create a wallet for this user
	err = wallet.Create(address, u.ID)
	require.NoError(t, err)

	// make sure it's loaded
	c := ljsonrpc.NewClient(address)
	require.NoError(t, err)
	assert.True(t, isWalletLoaded(t, c, sdkrouter.WalletID(u.ID)))

	// unload the wallet
	count, err := Unload(boil.GetDB(), 1*time.Minute)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	// make sure it got unloaded
	assert.False(t, isWalletLoaded(t, c, sdkrouter.WalletID(u.ID)))

	// reload it to make sure it can be loaded again in the future
	err = wallet.LoadWallet(address, u.ID)
	require.NoError(t, err)

	// make sure it got reloaded
	assert.True(t, isWalletLoaded(t, c, sdkrouter.WalletID(u.ID)))
}

func TestLegacyMiddleware(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/v1/proxy", nil)
	require.NoError(t, err)
	r.Header.Add("X-Lbry-Auth-Token", "auth me")

	authProvider := func(token, ip string) (*models.User, error) {
		return &models.User{ID: 994}, nil
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}

	rr := httptest.NewRecorder()
	hook := logrusTest.NewLocal(GetLogger().Entry.Logger)
	GetLogger().Entry.Logger.SetLevel(logrus.TraceLevel)

	auth.LegacyMiddleware(authProvider)(
		Middleware(boil.GetDB())(
			http.HandlerFunc(handler),
		),
	).ServeHTTP(rr, r)
	body, err := ioutil.ReadAll(rr.Result().Body)
	require.NoError(t, err)
	assert.Equal(t, string(body), "hello")

	assert.Equal(t, 1, len(hook.Entries), "unexpected log entry")
	assert.Equal(t, logrus.TraceLevel, hook.LastEntry().Level)
	assert.Equal(t, "touched user", hook.LastEntry().Message)
	assert.Equal(t, 994, hook.LastEntry().Data["user_id"])
}

func TestMiddleware(t *testing.T) {
	r, err := http.NewRequest("GET", "/api/v1/proxy", nil)
	require.NoError(t, err)
	r.Header.Add("Authorization", "auth me")

	authProvider := func(token, ip string) (*models.User, error) {
		return &models.User{ID: 994, IdpID: null.StringFrom("my-idp-id")}, nil
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}

	rr := httptest.NewRecorder()
	hook := logrusTest.NewLocal(GetLogger().Entry.Logger)
	GetLogger().Entry.Logger.SetLevel(logrus.TraceLevel)

	auth.Middleware(authProvider)(
		Middleware(boil.GetDB())(
			http.HandlerFunc(handler),
		),
	).ServeHTTP(rr, r)
	body, err := ioutil.ReadAll(rr.Result().Body)
	require.NoError(t, err)
	assert.Equal(t, string(body), "hello")

	assert.Equal(t, 1, len(hook.Entries), "unexpected log entry")
	assert.Equal(t, logrus.TraceLevel, hook.LastEntry().Level)
	assert.Equal(t, "touched user", hook.LastEntry().Message)
	assert.Equal(t, 994, hook.LastEntry().Data["user_id"])
}

func isWalletLoaded(t *testing.T, c *ljsonrpc.Client, id string) bool {
	wallets, err := c.WalletList(id, 1, 1)
	if err != nil && strings.Contains(err.Error(), `Couldn't find wallet`) {
		return false
	}
	require.NoError(t, err)
	return len(wallets.Items) > 0
}
