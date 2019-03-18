package player

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// An MP4 file, size: 158433824 bytes, blobs: 77
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
	assert.Equal(t, 0, rs.SDBlob.BlobInfos[0].BlobNum)
	assert.Equal(t, 38, rs.SDBlob.BlobInfos[38].BlobNum)
}

func TestPlayURI_0B_52B(t *testing.T) {
	var err error
	r, _ := http.NewRequest("", "", nil)
	r.Header.Add("Range", "bytes=0-52")
	rr := httptest.NewRecorder()
	err = PlayURI(streamURL, rr, r)
	if err != nil {
		t.Error(err)
		return
	}
	response := rr.Result()
	if http.StatusPartialContent != response.StatusCode {
		t.Errorf("erroneous response status: %v", response.StatusCode)
		return
	}
	assert.Equal(t, "bytes", response.Header["Accept-Ranges"][0])
	assert.Equal(t, "video/mp4", response.Header["Content-Type"][0])

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

func TestPlayURI_156B_259B(t *testing.T) {
	var err error
	r, _ := http.NewRequest("", "", nil)
	r.Header.Add("Range", "bytes=156-259")
	rr := httptest.NewRecorder()
	err = PlayURI(streamURL, rr, r)
	if err != nil {
		t.Error(err)
		return
	}
	response := rr.Result()
	if http.StatusPartialContent != response.StatusCode {
		t.Errorf("erroneous response status: %v", response.StatusCode)
		return
	}
	assert.Equal(t, "bytes", response.Header["Accept-Ranges"][0])
	assert.Equal(t, "video/mp4", response.Header["Content-Type"][0])

	responseData := make([]byte, 10000)
	emptyData := make([]byte, 10000-104)
	n, err := response.Body.Read(responseData)
	if 104 != n {
		t.Errorf("expected to read 104 bytes, read %v", n)
		return
	}
	expectedData, err := hex.DecodeString(
		"00000001D39A07E8D39A07E80000000100000000008977680000" +
			"0000000000000000000000000000000100000000000000000000" +
			"0000000000010000000000000000000000000000400000000780" +
			"00000438000000000024656474730000001C656C737400000000")
	assert.Nil(t, err)
	assert.Equal(t, expectedData, responseData[:104])
	assert.Equal(t, responseData[104:], emptyData)
}

func TestPlayURI_4MB_4MB105B(t *testing.T) {
	var err error
	r, _ := http.NewRequest("", "", nil)
	r.Header.Add("Range", "bytes=4000000-4000104")
	rr := httptest.NewRecorder()
	err = PlayURI(streamURL, rr, r)
	if err != nil {
		t.Error(err)
		return
	}
	response := rr.Result()
	if http.StatusPartialContent != response.StatusCode {
		t.Errorf("erroneous response status: %v", response.StatusCode)
		return
	}
	assert.Equal(t, "bytes", response.Header["Accept-Ranges"][0])
	assert.Equal(t, "video/mp4", response.Header["Content-Type"][0])

	responseData := make([]byte, 10000)
	emptyData := make([]byte, 10000-106)
	n, err := response.Body.Read(responseData)
	if 105 != n {
		t.Errorf("expected to read 105 bytes, read %v", n)
		return
	}
	expectedData, err := hex.DecodeString(
		"6E81C93A90DD3A322190C8D608E29AA929867407596665097B5AE780412" +
			"61638A51C10BC26770AFFEF1533715FBD1428DCADEDC7BEA5D7A9C7D170" +
			"B71EF38E7138D24B0C7E86D791695EDAE1B88EDBE54F95C98EF3DCFD91D" +
			"A025C284EE37D8FEEA2EA84B76B9A22D3")
	assert.Nil(t, err)
	assert.Equal(t, expectedData, responseData[:105])
	assert.Equal(t, responseData[106:], emptyData)
}

func TestPlayURI_Big(t *testing.T) {
	var err error
	r, _ := http.NewRequest("", "", nil)
	r.Header.Add("Range", "bytes=0-100000")
	rr := httptest.NewRecorder()
	err = PlayURI(streamURL, rr, r)
	if err != nil {
		t.Error(err)
		return
	}
	response := rr.Result()
	if http.StatusPartialContent != response.StatusCode {
		t.Errorf("erroneous response status: %v", response.StatusCode)
		return
	}
	assert.Equal(t, "bytes", response.Header["Accept-Ranges"][0])
	assert.Equal(t, "video/mp4", response.Header["Content-Type"][0])

	responseData := make([]byte, 100000)
	n, err := response.Body.Read(responseData)
	if 100000 != n {
		t.Errorf("expected to read 100000 bytes, read %v", n)
		return
	}
}
