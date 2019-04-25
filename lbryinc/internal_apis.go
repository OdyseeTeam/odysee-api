package lbryinc

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/lbryio/lbryweb.go/config"
)

type IAPIResponse struct {
	Success bool                   `json:"success"`
	Error   *string                `json:"error"`
	Data    map[string]interface{} `json:"data"`
	Trace   []string               `json:"_trace,omitempty"`
}

// Call just takes an existing raw POST payload and sends it to internal-apis
// at the requested object/method (i.e. user/new)
func Call(object string, method string, payload io.ReadCloser) (response IAPIResponse, err error) {
	url := fmt.Sprintf(
		"%v%v/%v",
		config.Settings.GetString("InternalAPIs"),
		object, method,
	)
	req, err := http.NewRequest(http.MethodPost, url, payload)
	if err != nil {
		return response, err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(body, &response)
	return response, err
}
