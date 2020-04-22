package server

import (
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartAndServeUntilShutdown(t *testing.T) {
	server := NewServer("localhost:40080", sdkrouter.New(config.GetLbrynetServers()))
	server.Start()
	go server.ServeUntilShutdown()

	time.Sleep(100 * time.Millisecond)
	response, err := http.Get("http://localhost:40080/")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, http.StatusOK, response.StatusCode)
	server.stopChan <- syscall.SIGINT

	// Retry 10 times to give the server a chance to shut down
	for range [10]int{} {
		time.Sleep(100 * time.Millisecond)
		response, err = http.Get("http://localhost:40080/")
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

	server := NewServer("localhost:40080", sdkrouter.New(config.GetLbrynetServers()))
	server.Start()
	go server.ServeUntilShutdown()

	request, _ := http.NewRequest("OPTIONS", "http://localhost:40080/api/v1/proxy", nil)
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
