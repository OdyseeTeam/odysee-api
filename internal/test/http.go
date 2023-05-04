package test

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type HTTPTest struct {
	Name string

	Method string
	URL    string

	ReqBody    io.Reader
	ReqHeader  map[string]string
	RemoteAddr string

	Code        int
	ResBody     string
	ResHeader   map[string]string
	ResContains string
}

func (test *HTTPTest) Run(handler http.Handler, t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest(test.Method, test.URL, test.ReqBody)
	require.NoError(t, err)
	// req.RequestURI = test.URL
	req.RemoteAddr = test.RemoteAddr

	// Add headers
	for key, value := range test.ReqHeader {
		req.Header.Set(key, value)
	}

	req.Host = "odysee.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != test.Code {
		t.Errorf("Expected %v %s as status code (got %v %s)", test.Code, http.StatusText(test.Code), w.Code, http.StatusText(w.Code))
	}

	for key, value := range test.ResHeader {
		header := w.Header().Get(key)

		if value != header {
			t.Errorf("Expected '%s' as '%s' (got '%s')", value, key, header)
		}
	}

	if test.ResBody != "" && w.Body.String() != test.ResBody {
		t.Errorf("Expected '%s' as body (got '%s'", test.ResBody, w.Body.String())
	}

	if test.ResContains != "" && !strings.Contains(w.Body.String(), test.ResContains) {
		t.Errorf("Expected '%s' to be present in response (got '%s'", test.ResContains, w.Body.String())
	}

	return w
}

func (test *HTTPTest) RunHTTP(t *testing.T) *http.Response {
	t.Helper()
	request, err := http.NewRequest(test.Method, test.URL, test.ReqBody)
	require.NoError(t, err)

	for key, value := range test.ReqHeader {
		request.Header.Set(key, value)
	}

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	if response.StatusCode != test.Code {
		t.Errorf("Expected %v %s as status code (got %v %s)", test.Code, http.StatusText(test.Code), response.StatusCode, http.StatusText(response.StatusCode))
	}

	for key, value := range test.ResHeader {
		header := response.Header.Get(key)

		if value != header {
			t.Errorf("Expected '%s' as '%s' (got '%s')", value, key, header)
		}
	}

	if test.ResBody != "" && string(body) != test.ResBody {
		t.Errorf("Expected '%s' as body (got '%s'", test.ResBody, string(body))
	}

	if test.ResContains != "" && !strings.Contains(string(body), test.ResContains) {
		t.Errorf("Expected '%s' to be present in response (got '%s'", test.ResContains, string(body))
	}

	return response
}
