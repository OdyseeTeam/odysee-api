package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContentByURLNoPayment(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://localhost:40080/content/url", nil)
	r.URL.RawQuery = "pra-onde-vamos-em-2018-seguran-a-online#3a508cce1fda3b7c1a2502cb4323141d40a2cf0b"
	r.Header.Add("Range", "bytes=0-1023")
	rr := httptest.NewRecorder()
	http.HandlerFunc(ContentByURL).ServeHTTP(rr, r)

	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	_, err := rr.Body.ReadByte()
	assert.Equal(t, io.EOF, err)
}
