package iapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultServerAddress = "https://api.odysee.com"
	timeout              = 5 * time.Second
	headerForwardedFor   = "X-Forwarded-For"
	headerOauthToken     = "Authorization"
	paramLegacyToken     = "auth_token"
)

var APIError = errors.New("internal-api error")

// Client stores data about internal-apis call it is about to make.
type Client struct {
	options clientOptions
}

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

// ClientOpts allow to provide extra parameters to NewClient:
// - ServerAddress
// - RemoteIP — to forward the IP of a frontend client making the request
type clientOptions struct {
	server       string
	legacyToken  string
	oauthToken   string
	remoteIP     string
	extraHeaders map[string]string
	extraParams  map[string]string
	httpClient   httpClient
}

func WithLegacyToken(token string) func(options *clientOptions) {
	return func(options *clientOptions) {
		options.legacyToken = token
	}
}

func WithOAuthToken(token string) func(options *clientOptions) {
	return func(options *clientOptions) {
		options.oauthToken = token
	}
}

func WithServer(server string) func(options *clientOptions) {
	return func(options *clientOptions) {
		options.server = server
	}
}

func WithRemoteIP(remoteIP string) func(options *clientOptions) {
	return func(options *clientOptions) {
		options.remoteIP = remoteIP
	}
}
func WithExtraHeader(key, value string) func(options *clientOptions) {
	return func(options *clientOptions) {
		options.extraHeaders[key] = value
	}
}

func WithExtraParam(key, value string) func(options *clientOptions) {
	return func(options *clientOptions) {
		options.extraParams[key] = value
	}
}

func WithHTTPClient(client httpClient) func(options *clientOptions) {
	return func(options *clientOptions) {
		options.httpClient = client
	}
}

// NewClient returns a client instance for internal-apis. It requires authToken to be provided
// for authentication.
func NewClient(optionFuncs ...func(*clientOptions)) (*Client, error) {
	options := &clientOptions{
		server:       "https://api.odysee.com",
		extraHeaders: map[string]string{},
		extraParams:  map[string]string{},
		httpClient:   &http.Client{Timeout: timeout},
	}

	for _, optionFunc := range optionFuncs {
		optionFunc(options)
	}

	if options.remoteIP != "" {
		options.extraHeaders[headerForwardedFor] = options.remoteIP
	}
	if options.legacyToken != "" {
		options.extraParams[paramLegacyToken] = options.legacyToken
	} else if options.oauthToken != "" {
		options.extraHeaders[headerOauthToken] = "Bearer " + options.oauthToken
	} else {
		return nil, errors.New("either legacy or oauth token required")
	}

	c := &Client{options: *options}
	return c, nil
}

func (c Client) prepareParams(params map[string]string) url.Values {
	data := url.Values{}
	for k, v := range c.options.extraParams {
		data.Add(k, v)
	}
	for k, v := range params {
		data.Add(k, v)
	}
	return data
}

func (c Client) Call(path string, params map[string]string, target interface{}) error {
	r, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/%s", c.options.server, path),
		strings.NewReader(c.prepareParams(params).Encode()),
	)
	if err != nil {
		return err
	}

	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for k, v := range c.options.extraHeaders {
		r.Header.Set(k, v)
	}

	resp, err := c.options.httpClient.Do(r)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("server returned non-OK status: %v", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, target)
	if err != nil {
		return err
	}

	var bresp BaseResponse
	err = json.Unmarshal(body, &bresp)
	if err != nil {
		return err
	}
	if !bresp.Success {
		return fmt.Errorf("%w: %s", APIError, *bresp.Error)
	}

	return nil
}
