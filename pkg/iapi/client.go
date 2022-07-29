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

type CustomerListResponse struct {
	Success bool    `json:"success"`
	Error   *string `json:"error"`
	Data    []struct {
		ID               int       `json:"id"`
		TipperUserID     int       `json:"tipper_user_id"`
		CreatorUserID    int       `json:"creator_user_id"`
		AccountID        int       `json:"account_id"`
		ChannelName      string    `json:"channel_name"`
		ChannelClaimID   string    `json:"channel_claim_id"`
		TippedAmount     int       `json:"tipped_amount"`
		ReceivedAmount   int       `json:"received_amount"`
		Currency         string    `json:"currency"`
		TargetClaimID    string    `json:"target_claim_id"`
		Status           string    `json:"status"`
		PaymentIntentID  string    `json:"payment_intent_id"`
		PrivateTip       bool      `json:"private_tip"`
		CreatedAt        time.Time `json:"created_at"`
		UpdatedAt        time.Time `json:"updated_at"`
		Type             string    `json:"type"`
		ReferenceClaimID *string   `json:"reference_claim_id"`
		ValidThrough     time.Time `json:"valid_through"`
		SourceClaimID    string    `json:"source_claim_id"`
	} `json:"data"`
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
	return nil
}
