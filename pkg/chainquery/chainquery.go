package chainquery

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/friendsofgo/errors"
)

const (
	apiUrl       = "https://chainquery.odysee.tv/api/sql"
	queryTimeout = 15 * time.Second
)

type HttpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type HeightResponse struct {
	Success bool         `json:"success"`
	Error   *string      `json:"error"`
	Data    []HeightData `json:"data"`
}

type HeightData struct {
	Height int `json:"height"`
}

var client HttpDoer = &http.Client{
	Timeout: queryTimeout,
}

func GetHeight() (int, error) {
	r := HeightResponse{}
	err := makeRequest(client, "select height from block order by id desc limit 1", &r)
	if err != nil {
		return 0, errors.Wrap(err, "error retrieving latest height")
	}
	if len(r.Data) != 1 {
		return 0, errors.Errorf("error retrieving latest height, expected %v items in response, got %v", 1, len(r.Data))
	}
	return r.Data[0].Height, nil
}

func makeRequest(client HttpDoer, query string, target any) error {
	baseUrl, err := url.Parse(apiUrl)
	if err != nil {
		return err
	}
	params := url.Values{}
	params.Add("query", query)
	baseUrl.RawQuery = params.Encode()

	req, err := http.NewRequest(http.MethodGet, baseUrl.String(), nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: got %v, want %v", resp.StatusCode, http.StatusOK)
	}

	err = json.Unmarshal(body, target)
	if err != nil {
		return err
	}
	return nil
}
