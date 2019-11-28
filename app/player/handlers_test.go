package player

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rangeHeader struct {
	start, end, knownLen int
}

func makeRequest(method, uri string, rng *rangeHeader) *http.Response {
	router := mux.NewRouter()
	InstallRoutes(router)

	r, _ := http.NewRequest(method, uri, nil)
	if rng != nil {
		if rng.start == -1 {
			r.Header.Add("Range", fmt.Sprintf("bytes=-%v", rng.end))
		} else if rng.end == -1 {
			r.Header.Add("Range", fmt.Sprintf("bytes=%v-", rng.start))
		} else {
			r.Header.Add("Range", fmt.Sprintf("bytes=%v-%v", rng.start, rng.end))
		}
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, r)
	return rr.Result()
}

func TestHandleOptions(t *testing.T) {
	response := makeRequest(http.MethodOptions, "/content/claims/one/3ae4ed38414e426c29c2bd6aeab7a6ac5da74a98/stream.mp4", nil)

	assert.Equal(t, "video/mp4", response.Header.Get("Content-Type"))
	assert.Equal(t, "Sat, 27 Jul 2019 10:01:00 GMT", response.Header.Get("Last-Modified"))
	assert.Equal(t, "16499459", response.Header.Get("Content-Length"))
}

func TestHandleGet(t *testing.T) {
	uri := "/content/claims/what/6769855a9aa43b67086f9ff3c1a5bacb5698a27a/stream.mp4"
	type testInput struct {
		name, uri string
		rng       *rangeHeader
	}
	type testCase struct {
		input  testInput
		output string
	}
	testCases := []testCase{
		testCase{
			testInput{"MiddleBytes", uri, &rangeHeader{156, 259, 0}},
			"00000001D39A07E8D39A07E80000000100000000008977680000" +
				"0000000000000000000000000000000100000000000000000000" +
				"0000000000010000000000000000000000000000400000000780" +
				"00000438000000000024656474730000001C656C737400000000",
		},
		testCase{
			testInput{"FirstBytes", uri, &rangeHeader{0, 52, 0}},
			"00000018667479706D703432000000006D7034326D7034310000" +
				"C4EA6D6F6F760000006C6D76686400000000D39A07E8D39A07F200",
		},
		testCase{
			testInput{"BytesFromSecondBlob", uri, &rangeHeader{4000000, 4000104, 0}},
			"6E81C93A90DD3A322190C8D608E29AA929867407596665097B5AE780412" +
				"61638A51C10BC26770AFFEF1533715FBD1428DCADEDC7BEA5D7A9C7D170" +
				"B71EF38E7138D24B0C7E86D791695EDAE1B88EDBE54F95C98EF3DCFD91D" +
				"A025C284EE37D8FEEA2EA84B76B9A22D3",
		},
		testCase{
			testInput{"LastBytes", "/content/claims/known-size/0590f924bbee6627a2e79f7f2ff7dfb50bf2877c/stream", &rangeHeader{128791089, -1, 100}},
			"2505CA36CB47B0B14CA023203410E965657B6314F6005D51E992D073B8090419D49E28E99306C95CF2DDB9" +
				"51DA5FE6373AC542CC2D83EB129548FFA0B4FFE390EB56600AD72F0D517236140425E323FDFC649FDEB80F" +
				"A429227D149FD493FBCA2042141F",
		},
	}

	for _, row := range testCases {
		t.Run(row.input.uri, func(t *testing.T) {
			var expectedLen int
			response := makeRequest(http.MethodGet, row.input.uri, row.input.rng)

			if row.input.rng.knownLen > 0 {
				expectedLen = row.input.rng.knownLen
			} else {
				expectedLen = row.input.rng.end - row.input.rng.start + 1
			}
			require.Equal(t, http.StatusPartialContent, response.StatusCode)
			assert.Equal(t, fmt.Sprintf("%v", expectedLen), response.Header.Get("Content-Length"))
			assert.Equal(t, "bytes", response.Header.Get("Accept-Ranges"))
			assert.Equal(t, "video/mp4", response.Header.Get("Content-Type"))

			responseStream := make([]byte, expectedLen)
			_, err := response.Body.Read(responseStream)
			require.NoError(t, err)
			assert.Equal(t, strings.ToLower(row.output), hex.EncodeToString(responseStream))
		})
	}
}

func TestHandleOptionsErrors(t *testing.T) {
	r := makeRequest(http.MethodOptions, "/content/claims/completely/ef/stream", nil)
	require.Equal(t, http.StatusNotFound, r.StatusCode)
}

func TestHandleNotFound(t *testing.T) {
	r := makeRequest(http.MethodGet, "/content/claims/completely/ef/stream", nil)
	require.Equal(t, http.StatusNotFound, r.StatusCode)
}

func TestHandleOutOfBounds(t *testing.T) {
	r := makeRequest(http.MethodGet, "/content/claims/known-size/0590f924bbee6627a2e79f7f2ff7dfb50bf2877c/stream", &rangeHeader{999999999, -1, 0})

	require.Equal(t, http.StatusRequestedRangeNotSatisfiable, r.StatusCode)
}
