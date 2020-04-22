package wallet

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/lbrynet"
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
	defer connCleanup()

	code := m.Run()
	os.Exit(code)
}

func setupDBTables() {
	storage.Conn.Truncate([]string{"users"})
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
	setupDBTables()
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

func TestGetUserWithWallet_NonexistentUser(t *testing.T) {
	setupDBTables()

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
	assert.Equal(t, "cannot authenticate user with internal-apis: could not authenticate user", err.Error())
}

func TestGetUserWithWallet_ExistingUser(t *testing.T) {
	srv := test.RandServerAddress(t)
	rt := sdkrouter.New(map[string]string{"a": srv})
	setupDBTables()
	url, cleanup := dummyAPI(srv)
	defer cleanup()

	u, err := GetUserWithSDKServer(rt, url, "abc", "")
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.EqualValues(t, dummyUserID, u.ID)

	count, err := models.Users().CountG()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestGetUserWithWallet_ExistingUserWithoutSDKGetsAssignedOneOnRetrieve(t *testing.T) {
	setupDBTables()
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

	rt := sdkrouter.NewWithServers(srv)
	u, err := createDBUser(userID)
	require.NoError(t, err)
	require.NotNil(t, u)

	u, err = GetUserWithSDKServer(rt, ts.URL, "abc", "")
	require.NoError(t, err)
	assert.True(t, u.LbrynetServerID.Valid)
	assert.NotNil(t, u.R.LbrynetServer)
}

func TestGetUserWithWallet_NotVerifiedEmail(t *testing.T) {
	setupDBTables()

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
	setupDBTables()
	u := &models.User{ID: 4}
	u.LbrynetServerID.SetValid(55)
	rt := sdkrouter.New(config.GetLbrynetServers())
	l := logrus.NewEntry(logrus.New())
	err := assignSDKServerToUser(u, rt.RandomServer(), l)
	assert.EqualError(t, err, "user already has an sdk assigned")
}

func TestAssignSDKServerToUser_ConcurrentUpdates(t *testing.T) {
	setupDBTables()
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
	err = assignSDKServerToUser(u, s1, logger.Log())
	require.NoError(t, err)
	assert.True(t, u.LbrynetServerID.Valid)
	assert.Equal(t, s1.ID, u.LbrynetServerID.Int)

	// zero out assignment temporarily, and assign a different one
	u.LbrynetServerID = null.Int{}
	err = assignSDKServerToUser(u, s2, logger.Log())

	// check that it actually got reassigned the first one instead of the new one
	require.NoError(t, err)
	assert.True(t, u.LbrynetServerID.Valid)
	assert.Equal(t, s1.ID, u.LbrynetServerID.Int)
}

func BenchmarkWalletCommands(b *testing.B) {
	setupDBTables()

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
			res, err := cl.Call("account_balance", map[string]string{"wallet_id": u.WalletID})
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

	err = loadWallet(addr, userID)
	require.NoError(t, err)
}
