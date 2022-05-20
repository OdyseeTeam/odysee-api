package server

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/pkg/migrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
	storage.SetDB(db)
	// override this to temp to avoid permission error when running tests on
	// restricted environment.
	config.Config.Override("PublishSourceDir", os.TempDir())
	code := m.Run()
	dbCleanup()
	os.Exit(code)
}

func randomServer(r *sdkrouter.Router) *Server {
	return NewServer(fmt.Sprintf("localhost:%v", 30000+rand.Intn(30000)), r)
}

func TestStartAndServeUntilShutdown(t *testing.T) {
	server := randomServer(sdkrouter.New(config.GetLbrynetServers()))
	server.Start()
	go server.ServeUntilShutdown()

	url := fmt.Sprintf("http://%v/", server.Address())

	time.Sleep(500 * time.Millisecond)
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, http.StatusOK, response.StatusCode)
	server.stopChan <- syscall.SIGINT

	// Retry 10 times to give the server a chance to shut down
	for range [10]int{} {
		time.Sleep(100 * time.Millisecond)
		_, err = http.Get(url)
		if err != nil {
			break
		}
	}
	assert.Error(t, err)
}

func TestHeaders(t *testing.T) {
	var (
		err      error
		response *http.Response
	)

	server := randomServer(sdkrouter.New(config.GetLbrynetServers()))
	server.Start()
	go server.ServeUntilShutdown()

	url := fmt.Sprintf("http://%v/api/v1/proxy", server.Address())

	request, _ := http.NewRequest("OPTIONS", url, nil)
	client := http.Client{}

	// Retry 10 times to give the server a chance to start
	for range [10]int{} {
		time.Sleep(100 * time.Millisecond)
		response, err = client.Do(request)
		if err == nil {
			break
		}
	}

	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "*", response.Header.Get("Access-Control-Allow-Origin"))

	server.stopChan <- syscall.SIGINT
}
