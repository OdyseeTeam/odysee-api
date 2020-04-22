package proxy

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"
)

const testSetupWait = 200 * time.Millisecond

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())

	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}
	c, connCleanup := storage.CreateTestConn(params)
	c.SetDefaultConnection()

	defer connCleanup()

	os.Exit(m.Run())
}

func testFuncSetup() {
	storage.Conn.Truncate([]string{"users"})
	time.Sleep(testSetupWait)
}
