package iapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/OdyseeTeam/odysee-api/pkg/logging"
)

const (
	EnvironLive  = "live"
	EnvironTest  = "test"
	ParamEnviron = "environment"

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
	extraHeaders map[string]string
	extraParams  map[string]string
	httpClient   httpClient
}

func WithLegacyToken(token string) func(options *clientOptions) {
	return WithExtraParam(paramLegacyToken, token)
}

func WithOAuthToken(token string) func(options *clientOptions) {
	return WithExtraHeader(headerOauthToken, "Bearer "+token)
}

func WithServer(server string) func(options *clientOptions) {
	return func(options *clientOptions) {
		options.server = server
	}
}

func WithRemoteIP(remoteIP string) func(options *clientOptions) {
	return WithExtraHeader(headerForwardedFor, remoteIP)
}

func WithExtraHeader(key, value string) func(options *clientOptions) {
	return func(options *clientOptions) {
		options.extraHeaders[key] = value
	}
}

func WithEnvironment(name string) func(options *clientOptions) {
	return WithExtraParam(ParamEnviron, name)
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
		extraParams:  map[string]string{ParamEnviron: EnvironLive},
		httpClient:   &http.Client{Timeout: timeout},
	}

	for _, optionFunc := range optionFuncs {
		optionFunc(options)
	}

	if options.extraHeaders[headerOauthToken] == "" && options.extraParams[paramLegacyToken] == "" {
		return nil, errors.New("either legacy or oauth token required")
	}

	c := &Client{options: *options}
	return c, nil
}

func (c Client) Clone(optionFuncs ...func(*clientOptions)) *Client {
	o := c.options
	o.extraHeaders = map[string]string{}
	o.extraParams = map[string]string{}
	for k, v := range c.options.extraHeaders {
		o.extraHeaders[k] = v
	}
	for k, v := range c.options.extraParams {
		o.extraParams[k] = v
	}

	for _, optionFunc := range optionFuncs {
		optionFunc(&o)
	}
	return &Client{options: o}
}

func (c *Client) prepareParams(params map[string]string) url.Values {
	data := url.Values{}
	for k, v := range c.options.extraParams {
		data.Add(k, v)
	}
	for k, v := range params {
		data.Add(k, v)
	}
	return data
}

func (c *Client) Call(ctx context.Context, path string, params map[string]string, target interface{}) error {
	pp := c.prepareParams(params)
	lp := map[string]string{}
	for k := range pp {
		var v string
		if k == paramLegacyToken {
			v = "****"
		} else if len(pp[k]) > 0 {
			v = pp[k][0]
		}
		lp[k] = v
	}
	log := logging.FromContext(ctx).With("params", lp)

	r, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/%s", c.options.server, path),
		strings.NewReader(pp.Encode()),
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
		log.Debug("iapi request failed", "err", err)
		return err
	}
	if resp.StatusCode >= 500 {
		log.Debug("iapi server failure", "error_code", resp.StatusCode)
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
		log.Debug("iapi unmarshal failure", "err", err, "response_body", body)
		return err
	}

	if !bresp.Success {
		log.Debug("iapi error", "err", *bresp.Error)
		return fmt.Errorf("%w: %s", APIError, *bresp.Error)
	}

	log.Debug("iapi result", "object", bresp)
	return nil
}
