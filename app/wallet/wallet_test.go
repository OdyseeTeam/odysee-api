package wallet

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/lbrynet"
	"github.com/OdyseeTeam/odysee-api/internal/responses"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"

	"github.com/Pallinder/go-randomdata"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
)

const dummyUserID = 751365

func TestMain(m *testing.M) {
	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
	storage.SetDB(db)
	code := m.Run()
	dbCleanup()
	os.Exit(code)
}

func setupTest() {
	storage.Migrator.Truncate([]string{models.TableNames.Users})
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
	sdk := &models.LbrynetServer{
		Name:    randomdata.Alphanumeric(12),
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

	rt := sdkrouter.New(config.GetLbrynetServers())
	u, err := GetUserWithSDKServer(rt, config.GetInternalAPIHost(), "non-existent-token", "")
	require.Error(t, err)
	require.Nil(t, u)
	assert.EqualError(t, err, "internal-api error: you are not authorized to perform this action")
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
	err := u.Insert(storage.DB, boil.Infer())
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

func TestInitializeWallet(t *testing.T) {
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
	userID := rand.Int()
	addr := test.RandServerAddress(t)

	err := createWallet(addr, userID)
	require.NoError(t, err)

	err = createWallet(addr, userID)
	require.NotNil(t, err)
	assert.ErrorIs(t, err, lbrynet.ErrWalletAlreadyLoaded, err.Error())

	err = UnloadWallet(addr, userID)
	require.NoError(t, err)

	err = LoadWallet(addr, userID)
	require.NoError(t, err)
}

func TestCreateDBUser_ConcurrentDuplicateUser(t *testing.T) {
	setupTest()

	id := 123
	user := &models.User{ID: id}
	err := user.Insert(storage.DB, boil.Infer())
	require.NoError(t, err)

	// We want the very first getDBUser() call in getOrCreateLocalUser() to return no results to
	// simulate the case where that call returns nothing and then the user is created in another
	// request

	mockExecutor := &firstQueryNoResults{}

	err = inTx(context.Background(), storage.DB, func(tx *sql.Tx) error {
		mockExecutor.ex = tx
		_, err := getOrCreateLocalUser(mockExecutor, models.User{ID: id}, logger.Log())
		return err
	})

	assert.NoError(t, err)
}

func TestCreateDBUser_ConcurrentDuplicateUserIDP(t *testing.T) {
	setupTest()

	id := null.StringFrom("my-idp-id")
	user := &models.User{IdpID: id}
	err := user.Insert(storage.DB, boil.Infer())
	require.NoError(t, err)

	// we want the very first getDBUser() call in getOrCreateLocalUser() to return no results to
	// simulate the case where that call returns nothing and then the user is created in another
	// request

	mockExecutor := &firstQueryNoResults{}

	err = inTx(context.Background(), storage.DB, func(tx *sql.Tx) error {
		mockExecutor.ex = tx
		_, err := getOrCreateLocalUser(mockExecutor, models.User{IdpID: id}, logger.Log())
		return err
	})

	assert.NoError(t, err)
}

func TestDeleteUser(t *testing.T) {
	setupTest()

	id := 123
	user := &models.User{ID: id}
	err := user.Insert(storage.DB, boil.Infer())
	require.NoError(t, err)

	id2 := 124
	user2 := &models.User{ID: id2}
	err = user2.Insert(storage.DB, boil.Infer())
	require.NoError(t, err)

	require.NoError(t, DeleteUser(id))

	_, err = getDBUser(storage.DB, ByID(id))
	assert.EqualError(t, err, sql.ErrNoRows.Error())
	assert.EqualError(t, DeleteUser(id), sql.ErrNoRows.Error())

	_, err = getDBUser(storage.DB, ByID(id2))
	require.NoError(t, err)

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
