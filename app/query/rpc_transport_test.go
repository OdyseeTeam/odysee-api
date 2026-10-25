package query

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRPCTransportReusesConnectionsAcrossClients(t *testing.T) {
	var accepted atomic.Int64
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		fmt.Fprint(w, `{"jsonrpc":"2.0","result":true,"id":1}`)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			accepted.Add(1)
		}
	}
	server.Start()
	defer server.Close()
	transport := rpcTransport(7 * time.Second)
	defer transport.CloseIdleConnections()
	for i := 0; i < 100; i++ {
		// Caller creates a new RPC/HTTP client per call, so the transport must
		// carry the shared pool across those separate client objects.
		client := &http.Client{Transport: rpcTransport(7 * time.Second), Timeout: time.Second}
		res, err := client.Post(server.URL, "application/json", strings.NewReader(`{"method":"status"}`))
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := accepted.Load(); n != 1 {
		t.Fatalf("100 calls opened %d TCP connections; want 1", n)
	}
}

func TestRPCTransportConcurrentPoolCreation(t *testing.T) {
	var wg sync.WaitGroup
	results := make(chan *http.Transport, 64)
	for i := 0; i < cap(results); i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- rpcTransport(11 * time.Second) }()
	}
	wg.Wait()
	close(results)
	var first *http.Transport
	for transport := range results {
		if first == nil {
			first = transport
		}
		if transport != first {
			t.Fatal("concurrent callers received different connection pools")
		}
	}
}

func TestRPCTransportPreservesMethodHeaderTimeouts(t *testing.T) {
	fast := rpcTransport(20 * time.Millisecond)
	slow := rpcTransport(2 * time.Second)
	defer fast.CloseIdleConnections()
	defer slow.CloseIdleConnections()
	if fast == slow {
		t.Fatal("different method deadlines share a transport")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(120 * time.Millisecond):
			fmt.Fprint(w, "ok")
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	client := &http.Client{Transport: fast, Timeout: time.Second}
	res, err := client.Get(server.URL)
	if res != nil {
		res.Body.Close()
	}
	if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatalf("expected method header timeout, got %v", err)
	}
	client.Transport = slow
	res, err = client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil || string(body) != "ok" {
		t.Fatalf("longer deadline did not succeed: %q %v", body, err)
	}
}

func TestRPCTransportDoesNotMixEndpointOrRequestHeaders(t *testing.T) {
	transport := rpcTransport(13 * time.Second)
	defer transport.CloseIdleConnections()
	server := func(prefix string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "%s:%s", prefix, r.Header.Get("X-Wallet-Test"))
		}))
	}
	a, b := server("a"), server("b")
	defer a.Close()
	defer b.Close()
	for _, test := range []struct{ url, wallet, want string }{{a.URL, "one", "a:one"}, {b.URL, "two", "b:two"}, {a.URL, "three", "a:three"}} {
		request, _ := http.NewRequest(http.MethodPost, test.url, strings.NewReader("{}"))
		request.Header.Set("X-Wallet-Test", test.wallet)
		response, err := (&http.Client{Transport: transport, Timeout: time.Second}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || string(body) != test.want {
			t.Fatalf("wrong endpoint/request response: %q %v", body, err)
		}
	}
	if transport.IdleConnTimeout <= 0 || transport.MaxIdleConns <= 0 || transport.MaxIdleConnsPerHost <= 0 {
		t.Fatal("idle pools must expire and have finite capacity")
	}
}
