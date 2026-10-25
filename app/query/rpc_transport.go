package query

import (
	"net"
	"net/http"
	"sync"
	"time"
)

var rpcTransports = struct {
	sync.Mutex
	byTimeout map[time.Duration]*http.Transport
}{byTimeout: make(map[time.Duration]*http.Transport)}

// Share connection pools across callers while preserving each method's header
// timeout. Wallet identity and RPC headers remain on each individual request.
func rpcTransport(timeout time.Duration) *http.Transport {
	rpcTransports.Lock()
	defer rpcTransports.Unlock()

	if transport := rpcTransports.byTimeout[timeout]; transport != nil {
		return transport
	}
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 120 * time.Second,
		}).DialContext,
		MaxIdleConns:          512,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: timeout * 2,
	}
	rpcTransports.byTimeout[timeout] = transport
	return transport
}
