package server

import (
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/lbryio/lbryweb.go/config"
	"github.com/stretchr/testify/assert"
)

func TestStartAndServeUntilShutdown(t *testing.T) {
	config.Override("Address", "localhost:40080")
	defer config.RestoreOverridden()

	server := NewConfiguredServer()
	server.Start()
	go server.ServeUntilShutdown()

	response, err := http.Get("http://localhost:40080/")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, http.StatusOK, response.StatusCode)
	server.InterruptChan <- syscall.SIGINT

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
	config.Override("Address", "localhost:40080")
	defer config.RestoreOverridden()

	server := NewConfiguredServer()
	server.Start()
	go server.ServeUntilShutdown()

	response, err := http.Get("http://localhost:40080/")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "*", response.Header["Access-Control-Allow-Origin"][0])
	server.InterruptChan <- syscall.SIGINT
}
