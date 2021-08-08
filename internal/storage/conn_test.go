package storage

import (
	"database/sql"
	"os"
	"testing"

	"github.com/lbryio/lbry.go/v2/extras/crypto"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var testConn *Connection

func TestMain(m *testing.M) {
	dbConfig := config.GetDatabase()
	testConn = InitConn(ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	})
	testConn.Connect()
	defer testConn.Close()

	code := m.Run()

	os.Exit(code)
}

func TestInit(t *testing.T) {
	dbConfig := config.GetDatabase()
	params := ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}
	conn := InitConn(params)
	assert.Equal(t, params, conn.params)

	err := conn.Connect()
	if err != nil {
		t.Skipf("database server is down? skipping (%v)", err)
	}
	assert.NotNil(t, conn.DB)
	defer conn.Close()

	err = conn.DB.Ping()
	require.NoError(t, err)
}

func TestMigrate(t *testing.T) {
	var err error
	var rows *sql.Rows
	tempDbName := crypto.RandString(24)
	err = testConn.CreateDB(tempDbName)
	if err != nil {
		t.Fatal(err)
	}
	defer testConn.DropDB(tempDbName)

	c, err := testConn.SpawnConn(tempDbName)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	rows, err = c.DB.Query("SELECT id FROM users")
	require.Error(t, err)

	c.MigrateUp(0)
	rows, err = c.DB.Query("SELECT id FROM users")
	require.NoError(t, err)
	rows.Close()

	c.MigrateDown(0)
	rows, err = c.DB.Query("SELECT id FROM users")
	require.Error(t, err)
	require.Nil(t, rows)
}

func TestMakeDSN(t *testing.T) {
	assert.Equal(t,
		MakeDSN(ConnParams{Connection: "postgres://pg:pg@db", DBName: "test", Options: "sslmode=disable"}),
		"postgres://pg:pg@db/test?sslmode=disable",
	)
}
