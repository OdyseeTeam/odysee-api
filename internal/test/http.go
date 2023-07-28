package test

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	ReqBody     io.Reader
	ReqBodyJSON any
	ReqHeader   map[string]string
	RemoteAddr  string

	Code        int
	ResBody     string
	ResHeader   map[string]string
	ResContains string
}

func (test *HTTPTest) Run(handler http.Handler, t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	request := test.buildRequest(t)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, request)

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
	require := require.New(t)
	request := test.buildRequest(t)
	client := &http.Client{}
	response, err := client.Do(request)
	require.NoError(err)

	if response.StatusCode != test.Code {
		var details string
		if response.StatusCode == http.StatusNotFound {
			details = fmt.Sprintf(", url: %s", test.URL)
		} else {
			body, _ := ioutil.ReadAll(response.Body)
			details = fmt.Sprintf(", body: %s", body)
		}
		t.Errorf(
			"Expected %v %s as status code (got %v %s)%s",
			test.Code, http.StatusText(test.Code),
			response.StatusCode, http.StatusText(response.StatusCode),
			details,
		)
	}

	for key, value := range test.ResHeader {
		header := response.Header.Get(key)
		if value != header {
			t.Errorf("Expected '%s' as '%s' (got '%s')", value, key, header)
		}
	}

	if test.ResBody != "" || test.ResContains != "" {
		defer response.Body.Close()
		body, err := ioutil.ReadAll(response.Body)
		require.NoError(err)
		if test.ResBody != "" && string(body) != test.ResBody {
			t.Errorf("Expected '%s' as body (got '%s'", test.ResBody, string(body))
		}
		if test.ResContains != "" && !strings.Contains(string(body), test.ResContains) {
			t.Errorf("Expected '%s' to be present in response (got '%s'", test.ResContains, string(body))
		}
	}
	return response
}

func (test *HTTPTest) buildRequest(t *testing.T) *http.Request {
	t.Helper()

	var body io.Reader
	if test.ReqBody != nil {
		body = test.ReqBody
	} else if test.ReqBodyJSON != nil {
		b, err := json.Marshal(test.ReqBodyJSON)
		require.NoError(t, err)
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest(test.Method, test.URL, body)
	require.NoError(t, err)
	req.RemoteAddr = test.RemoteAddr

	if test.ReqBodyJSON != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range test.ReqHeader {
		req.Header.Set(key, value)
	}

	req.Host = "odysee.com"
	return req
}
