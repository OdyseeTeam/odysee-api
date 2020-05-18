package paid

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlePublicKeyRequest(t *testing.T) {
	r, err := http.NewRequest(http.MethodGet, "", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	HandlePublicKeyRequest(rr, r)

	response := rr.Result()
	key, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.NotZero(t, key)
	assert.Equal(t, km.PublicKeyBytes(), key)
}
