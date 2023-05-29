package server

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"

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
	return NewServer(fmt.Sprintf("localhost:%v", 30000+rand.Intn(30000)), r, nil)
}

func TestStartAndServeUntilShutdown(t *testing.T) {
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
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		response, err = client.Do(request)
		if err == nil {
			break
		}
	}

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "*", response.Header.Get("Access-Control-Allow-Origin"))
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	server.stopChan <- syscall.SIGINT

	// Retry 10 times to give the server a chance to shut down
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		_, err = http.Get(url)
		if err != nil {
			break
		}
	}
	assert.Error(t, err)
}
