package accesstracker

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/internal/test"
	"github.com/lbryio/lbrytv/models"
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
	c, connCleanup := storage.CreateTestConn(params)
	c.SetDefaultConnection()

	defer connCleanup()

	os.Exit(m.Run())
}

func TestTouch(t *testing.T) {
	// create user
	u := &models.User{ID: rand.Intn(99999)}
	err := u.InsertG(boil.Infer())
	require.NoError(t, err)

	// reload, check that access time is null
	err = u.ReloadG()
	require.NoError(t, err)
	assert.False(t, u.WalletAccessedAt.Valid)

	// set access time back in the past
	u.WalletAccessedAt = null.TimeFrom(TimeNow().Add(-1 * time.Hour))
	_, err = u.UpdateG(boil.Infer())
	require.NoError(t, err)

	// reload, check that access time is not null
	err = u.ReloadG()
	require.NoError(t, err)
	assert.True(t, u.WalletAccessedAt.Valid)

	// touch user
	Touch(boil.GetDB(), u.ID)

	// reload, check that access time has been updated
	err = u.ReloadG()
	require.NoError(t, err)
	assert.True(t, u.WalletAccessedAt.Valid)
	assert.True(t, TimeNow().Add(-1*time.Minute).Before(u.WalletAccessedAt.Time))
}

func TestUnload(t *testing.T) {
	reqChan := test.ReqChan()
	ts := test.MockHTTPServer(reqChan)
	defer ts.Close()
	ts.NextResponse <- `{"id":1}` // for the wallet.Unload call

	// create models
	l := &models.LbrynetServer{ID: rand.Intn(99999), Name: "x", Address: ts.URL}
	err := l.InsertG(boil.Infer())
	require.NoError(t, err)

	u := &models.User{
		ID:               rand.Intn(99999),
		LbrynetServerID:  null.IntFrom(l.ID),
		WalletAccessedAt: null.TimeFrom(TimeNow().Add(-5 * time.Minute)),
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
