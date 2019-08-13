package server

import (
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStartAndServeUntilShutdown(t *testing.T) {
	server := NewServer("localhost:40080")
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
	var (
		err      error
		response *http.Response
	)

	server := NewServer("localhost:40080")
	server.Start()
	go server.ServeUntilShutdown()

	// Retry 10 times to give the server a chance to start
	for range [10]int{} {
		time.Sleep(100 * time.Millisecond)
		response, err = http.Get("http://localhost:40080/")
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "*", response.Header["Access-Control-Allow-Origin"][0])
	assert.Equal(t, "X-Lbry-Auth-Token, Origin, X-Requested-With, Content-Type, Accept", response.Header["Access-Control-Allow-Headers"][0])
	server.InterruptChan <- syscall.SIGINT
}
