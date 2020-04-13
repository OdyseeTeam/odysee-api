package proxy

import (
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/responses"
	"github.com/lbryio/lbrytv/internal/storage"
)

const dummyUserID = 751365
const dummyServerURL = "http://127.0.0.1:59999"
const proxySuffix = "/api/v1/proxy"
const testSetupWait = 200 * time.Millisecond

var svc *Service

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())

	svc = NewService(sdkrouter.New(config.GetLbrynetServers()))

	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}
	c, connCleanup := storage.CreateTestConn(params)
	c.SetDefaultConnection()

	defer connCleanup()

	code := m.Run()

	os.Exit(code)
}

func testFuncSetup() {
	storage.Conn.Truncate([]string{"users"})
	time.Sleep(testSetupWait)
}

func testFuncTeardown() {

}

func launchDummyAPIServer(response []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responses.AddJSONContentType(w)
		w.Write(response)
	}))
}

func launchDummyAPIServerDelayed(response []byte, delayMsec time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delayMsec * time.Millisecond)
		responses.AddJSONContentType(w)
		w.Write(response)
	}))
}
