package server

import (
	"net/http"
	"syscall"
	"testing"

	"github.com/lbryio/lbryweb.go/config"
	"github.com/stretchr/testify/assert"
)

func TestStartAndWaitForShutdown(t *testing.T) {
	config.Override("Address", "localhost:40080")
	defer config.RestoreOverridden()

	server := NewConfiguredServer()
	server.Start()
	go server.WaitForShutdown()

	response, err := http.Get("http://localhost:40080/")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, http.StatusOK, response.StatusCode)
	server.InterruptChan <- syscall.SIGINT

	response, err = http.Get("http://localhost:40080/")
	assert.Error(t, err)
}
func TestHeaders(t *testing.T) {
	config.Override("Address", "localhost:40080")
	defer config.RestoreOverridden()

	server := NewConfiguredServer()
	server.Start()
	go server.WaitForShutdown()

	response, err := http.Get("http://localhost:40080/")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "*", response.Header["Access-Control-Allow-Origin"][0])
	server.InterruptChan <- syscall.SIGINT
}
