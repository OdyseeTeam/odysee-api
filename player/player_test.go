package player

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

const streamURL = "what#6769855a9aa43b67086f9ff3c1a5bacb5698a27a"

func Test_newReflectedStream(t *testing.T) {
	rs, err := newReflectedStream(streamURL)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t,
		"d5169241150022f996fa7cd6a9a1c421937276a3275eb912790bd07ba7aec1fac5fd45431d226b8fb402691e79aeb24b",
		string(rs.SDHash))
}

func Test_newReflectedStream_emptyURL(t *testing.T) {
	_, err := newReflectedStream("")
	assert.NotNil(t, err)
}

func TestReflectedStream_fetchData(t *testing.T) {
	rs, err := newReflectedStream(streamURL)
	err = rs.fetchData()
	assert.Nil(t, err)
	assert.NotNil(t, rs.SDHash)
	assert.Equal(t, 0, rs.Blobs.BlobInfos[0].BlobNum)
	assert.Equal(t, 38, rs.Blobs.BlobInfos[38].BlobNum)
}

func TestReflectedStream_prepareWriter(t *testing.T) {
	rs, _ := newReflectedStream(streamURL)
	rr := httptest.NewRecorder()
	rs.fetchData()
	blobStart, blobEnd := rs.prepareWriter(5*1024*1024, 15*1024*1024, rr)
	response := rr.Result()
	assert.Equal(t, 2, blobStart)
	assert.Equal(t, 7, blobEnd)
	assert.Equal(t, http.StatusPartialContent, response.StatusCode)
	assert.Equal(t, "bytes", response.Header["Accept-Ranges"][0])
	assert.Equal(t, "video/mp4", response.Header["Content-Type"][0])
	assert.Equal(t, "12582907", response.Header["Content-Length"][0])
	assert.Equal(t, "bytes 4194302-16777208/158433814", response.Header["Content-Range"][0])
}

func Test_parseRange(t *testing.T) {
	var start, end int64

	start, end = parseRange("range=0-111111")
	assert.EqualValues(t, 0, start)
	assert.EqualValues(t, 0, end)

	start, end = parseRange("range=")
	assert.EqualValues(t, 0, start)
	assert.EqualValues(t, 0, end)

	start, end = parseRange("")
	assert.EqualValues(t, 0, start)
	assert.EqualValues(t, 0, end)

	start, end = parseRange("bytes-")
	assert.EqualValues(t, 0, start)
	assert.EqualValues(t, 0, end)

	start, end = parseRange("bytes=124-1")
	assert.EqualValues(t, 0, start)
	assert.EqualValues(t, 0, end)

	start, end = parseRange("bytes=15-124")
	assert.Equal(t, int64(15), start)
	assert.Equal(t, int64(124), end)
}

func TestPlayURI(t *testing.T) {
	var err error
	rr := httptest.NewRecorder()
	PlayURI(streamURL, "bytes=0-52", rr)
	if err != nil {
		t.Error(err)
		return
	}
	response := rr.Result()
	responseFirst52 := make([]byte, 52)
	n, err := response.Body.Read(responseFirst52)
	if 52 != n {
		t.Errorf("expected to read 52 bytes, read %v", n)
		return
	}
	first52, err := hex.DecodeString(
		"00000018667479706D703432000000006D7034326D7034310000" +
			"C4EA6D6F6F760000006C6D76686400000000D39A07E8D39A07F2")
	assert.Nil(t, err)
	assert.Equal(t, first52, responseFirst52)
}
