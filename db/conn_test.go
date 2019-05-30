package db

import (
	"database/sql"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func TestInit(t *testing.T) {
	c := NewConnection(GetDSN(ConnParams{}))
	err := c.DB.Ping()
	require.Nil(t, err)
}

func TestMigrate(t *testing.T) {
	var err error
	var rows *sql.Rows
	tempDbName := RandStringBytes(24)
	err = CreateDB(tempDbName)
	if err != nil {
		t.Fatal(err)
	}
	c := NewConnection(GetDSN(ConnParams{DatabaseName: tempDbName}))

	rows, err = c.DB.Query("SELECT id FROM users")
	require.NotNil(t, err)

	c.MigrateUp()
	rows, err = c.DB.Query("SELECT id FROM users")
	require.Nil(t, err)
	rows.Close()

	c.MigrateDown()
	rows, err = c.DB.Query("SELECT id FROM users")
	require.NotNil(t, err)
	rows.Close()

	if err = c.DB.Close(); err != nil {
		t.Fatal(err)
	}
	err = DropDB(tempDbName)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetDSN(t *testing.T) {
	assert.Equal(t,
		GetDSN(ConnParams{}),
		"postgres://postgres:postgres@localhost/lbrytv?sslmode=disable",
	)
	assert.Equal(t,
		GetDSN(ConnParams{DatabaseName: "test"}),
		"postgres://postgres:postgres@localhost/test?sslmode=disable",
	)
	assert.Equal(t,
		GetDSN(ConnParams{DatabaseConnection: "postgres://pg:pg@db", DatabaseName: "test"}),
		"postgres://pg:pg@db/test?sslmode=disable",
	)
	assert.Equal(t,
		GetDSN(ConnParams{DatabaseOptions: "sslmode=enable"}),
		"postgres://postgres:postgres@localhost/lbrytv?sslmode=enable",
	)

}
