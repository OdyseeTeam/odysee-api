package arweave

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	batchResolverUrl      = "https://migrator.arfleet.zephyrdev.xyz/batch-resolve"
	batchClaimResolverUrl = "https://migrator.arfleet.zephyrdev.xyz/claims/batch-resolve"
	resolverTimeout       = 10 * time.Second

	paramDataItemId = "data_item_id"
)

type HttpDoer interface {
	Do(req *http.Request) (res *http.Response, err error)
}

type ArfleetResolver struct {
	baseUrl               string
	batchResolverUrl      string
	batchClaimResolverUrl string
	client                HttpDoer
}

type BatchResolveUrlResponse map[string]ResolveUrlResponse

type ResolveUrlResponse struct {
	URL        string `json:"url"`
	URLHash    string `json:"url_hash"`
	Arfleet    string `json:"arfleet"`
	DataItemId string `json:"data_item_id"`
	Resolved   bool   `json:"resolved"`
}

type BatchResolveClaimResponse map[string]ResolveClaimResponse

type ResolveClaimResponse struct {
	ClaimId    string `json:"claim_id"`
	Arfleet    string `json:"arfleet"`
	DataItemId string `json:"data_item_id"`
	Resolved   bool   `json:"resolved"`
}

func NewArfleetResolver(baseUrl string) *ArfleetResolver {
	r := &ArfleetResolver{
		baseUrl:               baseUrl,
		batchResolverUrl:      batchResolverUrl,
		batchClaimResolverUrl: batchClaimResolverUrl,
		client: &http.Client{
			Timeout: resolverTimeout,
		},
	}
	return r
}

func (c *ArfleetResolver) ResolveUrls(urls []string) (map[string]string, error) {
	substitutes := map[string]string{}

	jsonData, err := json.Marshal(map[string][]string{"urls": urls})
	if err != nil {
		return nil, err
	}

	jsonResponse, err := c.makeRequest(http.MethodPost, c.batchResolverUrl, jsonData)
	if err != nil {
		return nil, err
	}
	var resolvedList BatchResolveUrlResponse
	err = json.Unmarshal(jsonResponse, &resolvedList)
	if err != nil {
		return nil, fmt.Errorf("error parsing json: %w", err)
	}

	for url, resolved := range resolvedList {
		if !resolved.Resolved {
			continue
		}
		u, err := appendParams(resolved.Arfleet, map[string]string{paramDataItemId: resolved.DataItemId})
		if err != nil {
			continue
		}
		substitutes[url] = c.baseUrl + u
	}
	return substitutes, nil
}

func (c *ArfleetResolver) ResolveClaims(claim_ids []string) (map[string]string, error) {
	substitutes := map[string]string{}

	jsonData, err := json.Marshal(map[string][]string{"claim_ids": claim_ids})
	if err != nil {
		return nil, err
	}

	jsonResponse, err := c.makeRequest(http.MethodPost, c.batchClaimResolverUrl, jsonData)
	if err != nil {
		return nil, err
	}
	var resolvedList BatchResolveClaimResponse
	err = json.Unmarshal(jsonResponse, &resolvedList)
	if err != nil {
		return nil, fmt.Errorf("error parsing json: %w", err)
	}

	for claim, resolved := range resolvedList {
		if !resolved.Resolved {
			continue
		}
		u, err := appendParams(resolved.Arfleet, map[string]string{paramDataItemId: resolved.DataItemId})
		if err != nil {
			continue
		}
		substitutes[claim] = c.baseUrl + u
	}
	return substitutes, nil
}

func (c *ArfleetResolver) makeRequest(method, url string, jsonData []byte) ([]byte, error) {
	client := c.client

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: got %v, want %v", resp.StatusCode, http.StatusOK)
	}
	return body, nil
}

func appendParams(baseUrl string, params map[string]string) (string, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
