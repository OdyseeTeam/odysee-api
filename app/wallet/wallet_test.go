package wallet

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/responses"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/internal/test"
	"github.com/lbryio/lbrytv/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/ybbus/jsonrpc"
)

const dummyUserID = 751365

func TestMain(m *testing.M) {
	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}
	dbConn, connCleanup := storage.CreateTestConn(params)
	dbConn.SetDefaultConnection()

	code := m.Run()

	connCleanup()
	os.Exit(code)
}

func setupTest() {
	storage.Conn.Truncate([]string{"users"})
	currentCache.flush()
}

func dummyAPI(sdkAddress string) (string, func()) {
	reqChan := test.ReqChan()
	ts := test.MockHTTPServer(reqChan)
	go func() {
		for {
			req := <-reqChan
			responses.AddJSONContentType(req.W)
			ts.NextResponse <- fmt.Sprintf(`{
				"success": true,
				"error": null,
				"data": {
				  "user_id": %d,
				  "has_verified_email": true
				}
			}`, dummyUserID)
		}
	}()

	return ts.URL, func() {
		ts.Close()
		UnloadWallet(sdkAddress, dummyUserID)
	}
}

func TestGetUserWithWallet_NewUser(t *testing.T) {
	setupTest()
	srv := test.RandServerAddress(t)
	rt := sdkrouter.New(map[string]string{"a": srv})
	url, cleanup := dummyAPI(srv)
	defer cleanup()

	u, err := GetUserWithSDKServer(rt, url, "abc", "")
	require.NoError(t, err, errors.Unwrap(err))
	require.NotNil(t, u)

	count, err := models.Users(models.UserWhere.ID.EQ(u.ID)).CountG()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
	assert.True(t, u.LbrynetServerID.IsZero()) // because the server came from a config, it should not have an id set

	// now assign the user a new server thats set in the db
	//      rand.Intn(99999),
	sdk := &models.LbrynetServer{
		Name:    "testing",
		Address: "test.test.test.test",
	}
	err = u.SetLbrynetServerG(true, sdk)
	require.NoError(t, err)
	require.NotEqual(t, 0, sdk.ID)
	require.Equal(t, u.LbrynetServerID.Int, sdk.ID)

	// now fetch it all back from the db

	u2, err := GetUserWithSDKServer(rt, url, "abc", "")
	require.NoError(t, err, errors.Unwrap(err))
	require.NotNil(t, u2)

	sdk2, err := u.LbrynetServer().OneG()
	require.NoError(t, err)
	require.Equal(t, sdk.ID, sdk2.ID)
	require.Equal(t, sdk.Address, sdk2.Address)
	require.Equal(t, u.LbrynetServerID.Int, sdk2.ID)
}

func TestGetUserWithWallet_NewUserSDKError(t *testing.T) {
	setupTest()
	srv := test.RandServerAddress(t)
	sdk := &models.LbrynetServer{
		Name:    "failing",
		Address: "http://failure.test",
	}
	defer func() { sdk.DeleteG() }()
	rt := sdkrouter.NewWithServers(sdk)

	url, cleanup := dummyAPI(srv)
	defer cleanup()

	_, err := GetUserWithSDKServer(rt, url, "abc", "")
	assert.Regexp(t, `.+dial tcp: lookup failure.test`, err.Error())

	count, err := models.Users(models.UserWhere.ID.EQ(dummyUserID)).CountG()
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

func TestGetUserWithWallet_NonexistentUser(t *testing.T) {
	setupTest()

	ts := test.MockHTTPServer(nil)
	defer ts.Close()
	ts.NextResponse <- `{
		"success": false,
		"error": "could not authenticate user",
		"data": null
	}`

	rt := sdkrouter.New(config.GetLbrynetServers())
	u, err := GetUserWithSDKServer(rt, ts.URL, "non-existent-token", "")
	require.Error(t, err)
	require.Nil(t, u)
	assert.EqualError(t, err, "api error: could not authenticate user")
}

func TestGetUserWithWallet_ExistingUser(t *testing.T) {
	setupTest()
	srv := test.RandServerAddress(t)
	rt := sdkrouter.New(map[string]string{"a": srv})
	url, cleanup := dummyAPI(srv)
	defer cleanup()

	u, err := GetUserWithSDKServer(rt, url, "abc", "")
	assert.NoError(t, err)
	assert.NotNil(t, u)
	assert.EqualValues(t, dummyUserID, u.ID)

	count, err := models.Users().CountG()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestGetUserWithWallet_CachedUser(t *testing.T) {
	setupTest()
	srv := test.RandServerAddress(t)
	rt := sdkrouter.New(map[string]string{"a": srv})
	url, cleanup := dummyAPI(srv)
	defer cleanup()

	token := "abc"
	metricValue := metrics.GetCounterValue(metrics.AuthTokenCacheHits)

	u, err := GetUserWithSDKServer(rt, url, token, "")
	assert.NoError(t, err)
	assert.NotNil(t, u)
	assert.EqualValues(t, dummyUserID, u.ID)

	assert.Equal(t, metricValue, metrics.GetCounterValue(metrics.AuthTokenCacheHits))

	u, err = GetUserWithSDKServer(rt, url, token, "")
	assert.NoError(t, err)
	assert.NotNil(t, u)
	assert.EqualValues(t, dummyUserID, u.ID)
	assert.Equal(t, metricValue+1, metrics.GetCounterValue(metrics.AuthTokenCacheHits))
}

func TestGetUserWithWallet_ExistingUserWithoutSDKGetsAssignedOneOnRetrieve(t *testing.T) {
	setupTest()
	userID := rand.Intn(999999)

	reqChan := test.ReqChan()
	ts := test.MockHTTPServer(reqChan)
	defer ts.Close()
	go func() {
		req := <-reqChan
		responses.AddJSONContentType(req.W)
		ts.NextResponse <- fmt.Sprintf(`{
			"success": true,
			"error": null,
			"data": {
			  "user_id": %d,
			  "has_verified_email": true
			}
		}`, userID)
	}()

	srv := &models.LbrynetServer{Name: "a", Address: test.RandServerAddress(t)}
	srv.InsertG(boil.Infer())
	defer func() { srv.DeleteG() }()

	u := &models.User{ID: userID}
	err := u.Insert(storage.Conn.DB, boil.Infer())
	require.NoError(t, err)
	assert.False(t, u.CreatedAt.IsZero())

	rt := sdkrouter.NewWithServers(srv)
	u, err = GetUserWithSDKServer(rt, ts.URL, "abc", "")
	require.NoError(t, err)
	assert.True(t, u.LbrynetServerID.Valid)
	assert.NotNil(t, u.R.LbrynetServer)
}

func TestGetUserWithWallet_NotVerifiedEmail(t *testing.T) {
	setupTest()

	ts := test.MockHTTPServer(nil)
	defer ts.Close()
	ts.NextResponse <- `{
		"success": true,
		"error": null,
		"data": {
		  "user_id": 111,
		  "has_verified_email": false
		}
	}`

	rt := sdkrouter.New(config.GetLbrynetServers())
	u, err := GetUserWithSDKServer(rt, ts.URL, "abc", "")
	assert.NoError(t, err)
	assert.Nil(t, u)
}

func TestAssignSDKServerToUser_SDKAlreadyAssigned(t *testing.T) {
	setupTest()
	u := &models.User{ID: 4}
	u.LbrynetServerID.SetValid(55)
	rt := sdkrouter.New(config.GetLbrynetServers())
	l := logrus.NewEntry(logrus.New())
	err := assignSDKServerToUser(boil.GetDB(), u, rt.RandomServer(), l)
	assert.EqualError(t, err, "user already has an sdk assigned")
}

func TestAssignSDKServerToUser_ConcurrentUpdates(t *testing.T) {
	setupTest()
	ts := test.MockHTTPServer(nil)
	ts.NextResponse <- `{"id":1,"result":{"id":99,"name":"x.99.wallet"}}`

	s1 := &models.LbrynetServer{Name: "a", Address: ts.URL}
	err := s1.InsertG(boil.Infer())
	require.NoError(t, err)
	s2 := &models.LbrynetServer{Name: "b", Address: ts.URL}
	err = s2.InsertG(boil.Infer())
	require.NoError(t, err)

	u := &models.User{ID: rand.Intn(999999)}
	err = u.InsertG(boil.Infer())
	require.NoError(t, err)
	err = u.ReloadG()
	require.NoError(t, err)

	defer func() {
		u.DeleteG()
		s1.DeleteG()
		s2.DeleteG()
	}()

	// assign one sdk
	err = assignSDKServerToUser(boil.GetDB(), u, s1, logger.Log())
	require.NoError(t, err)
	assert.True(t, u.LbrynetServerID.Valid)
	assert.Equal(t, s1.ID, u.LbrynetServerID.Int)

	// zero out assignment temporarily, and assign a different one
	u.LbrynetServerID = null.Int{}
	err = assignSDKServerToUser(boil.GetDB(), u, s2, logger.Log())

	// check that it actually got reassigned the first one instead of the new one
	require.NoError(t, err)
	assert.True(t, u.LbrynetServerID.Valid)
	assert.Equal(t, s1.ID, u.LbrynetServerID.Int)
}

func BenchmarkWalletCommands(b *testing.B) {
	setupTest()

	reqChan := test.ReqChan()
	ts := test.MockHTTPServer(reqChan)
	defer ts.Close()
	go func() {
		req := <-reqChan
		responses.AddJSONContentType(req.W)
		ts.NextResponse <- fmt.Sprintf(`{
			"success": true,
			"error": null,
			"data": {
			  "user_id": %v,
			  "has_verified_email": true
			}
		}`, req.R.PostFormValue("auth_token"))
	}()

	walletsNum := 60
	users := make([]*models.User, walletsNum)
	rt := sdkrouter.New(config.GetLbrynetServers())
	cl := jsonrpc.NewClient(rt.RandomServer().Address)

	logger.Disable()
	sdkrouter.DisableLogger()
	logrus.SetOutput(ioutil.Discard)

	rand.Seed(time.Now().UnixNano())

	for i := 0; i < walletsNum; i++ {
		uid := rand.Intn(9999999)
		u, err := GetUserWithSDKServer(rt, ts.URL, fmt.Sprintf("%d", uid), "")
		require.NoError(b, err, errors.Unwrap(err))
		require.NotNil(b, u)
		users[i] = u
	}

	b.SetParallelism(20)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			u := users[rand.Intn(len(users))]
			res, err := cl.Call("account_balance", map[string]string{"wallet_id": sdkrouter.WalletID(u.ID)})
			require.NoError(b, err)
			assert.Nil(b, res.Error)
		}
	})

	b.StopTimer()
}

func TestCreate_CorrectWalletID(t *testing.T) {
	// TODO: test that calling Create() sends the correct wallet id to the server
}

func TestInitializeWallet(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	userID := rand.Int()
	addr := test.RandServerAddress(t)

	err := Create(addr, userID)
	require.NoError(t, err)

	err = UnloadWallet(addr, userID)
	require.NoError(t, err)

	err = Create(addr, userID)
	require.NoError(t, err)
}

func TestCreateWalletLoadWallet(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	userID := rand.Int()
	addr := test.RandServerAddress(t)

	err := createWallet(addr, userID)
	require.NoError(t, err)

	err = createWallet(addr, userID)
	require.NotNil(t, err)
	assert.True(t, errors.Is(err, lbrynet.ErrWalletExists))

	err = UnloadWallet(addr, userID)
	require.NoError(t, err)

	err = LoadWallet(addr, userID)
	require.NoError(t, err)
}

func TestCreateDBUser_ConcurrentDuplicateUser(t *testing.T) {
	storage.Conn.Truncate([]string{models.TableNames.Users})

	id := 123
	user := &models.User{ID: id}
	err := user.Insert(storage.Conn.DB.DB, boil.Infer())
	require.NoError(t, err)

	// we want the very first getDBUser() call in getOrCreateLocalUser() to return no results to
	// simulate the case where that call returns nothing and then the user is created in another
	// request

	mockExecutor := &firstQueryNoResults{}

	err = inTx(context.Background(), storage.Conn.DB.DB, func(tx *sql.Tx) error {
		mockExecutor.ex = tx
		_, err := getOrCreateLocalUser(mockExecutor, models.User{ID: id}, logger.Log())
		return err
	})

	assert.NoError(t, err)
}

func TestCreateDBUser_ConcurrentDuplicateUserIDP(t *testing.T) {
	storage.Conn.Truncate([]string{models.TableNames.Users})

	id := null.StringFrom("my-idp-id")
	user := &models.User{IdpID: id}
	err := user.Insert(storage.Conn.DB.DB, boil.Infer())
	require.NoError(t, err)

	// we want the very first getDBUser() call in getOrCreateLocalUser() to return no results to
	// simulate the case where that call returns nothing and then the user is created in another
	// request

	mockExecutor := &firstQueryNoResults{}

	err = inTx(context.Background(), storage.Conn.DB.DB, func(tx *sql.Tx) error {
		mockExecutor.ex = tx
		_, err := getOrCreateLocalUser(mockExecutor, models.User{IdpID: id}, logger.Log())
		return err
	})

	assert.NoError(t, err)
}

type firstQueryNoResults struct {
	ex    boil.Executor
	calls int
}

func (m *firstQueryNoResults) Exec(query string, args ...interface{}) (sql.Result, error) {
	return m.ex.Exec(query, args...)
}
func (m *firstQueryNoResults) Query(query string, args ...interface{}) (*sql.Rows, error) {
	m.calls++
	if m.calls == 1 {
		return nil, errors.Err(sql.ErrNoRows)
	}
	return m.ex.Query(query, args...)
}
func (m *firstQueryNoResults) QueryRow(query string, args ...interface{}) *sql.Row {
	m.calls++
	if m.calls == 1 {
		return m.ex.QueryRow("SELECT 0 <> 0") // just want something with no rows
	}
	return m.ex.QueryRow(query, args...)
}
