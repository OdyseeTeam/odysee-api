package chainquery

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failureTestClient struct{}

func (_ failureTestClient) Do(*http.Request) (*http.Response, error) {
	data := HeightResponse{
		Success: true,
		Error:   nil,
		Data:    []HeightData{},
	}

	body, _ := json.Marshal(data)

	return &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func TestGetHeight(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	height, err := GetHeight()
	require.NoError(err)
	assert.Greater(height, 1500_000)
}

func TestGetHeightFailure(t *testing.T) {
	assert := assert.New(t)

	origClient := client
	client = failureTestClient{}
	defer func() {
		client = origClient
	}()

	height, err := GetHeight()
	assert.ErrorContains(err, "error retrieving latest height, expected 1 items in response, got 0")
	assert.Equal(0, height)
}
