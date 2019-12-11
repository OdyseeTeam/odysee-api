package player

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lbryio/lbry.go/v2/stream"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/reflector.go/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	url := "understanding-anger-how-to-integrate#2940943d14402e9d92f2fd608613db6904f694c4"
	p := NewPlayer(nil)

	bStore := peer.NewStore(peer.StoreOpts{
		Address: config.GetRefractorAddress(),
		Timeout: time.Second * time.Duration(config.GetRefractorTimeout()),
	})

	// This is just to resolve the stream hash
	s, err := p.ResolveStream(url)
	require.NoError(t, err)

	// Purely lbry.go / reflector.go code below for isolation
	sdBlob := stream.SDBlob{}
	blob, err := bStore.Get(s.Hash)
	err = sdBlob.FromBlob(blob)
	require.NoError(t, err)

	hash := hex.EncodeToString(sdBlob.BlobInfos[0].BlobHash)

	blob, err = bStore.Get(hash)
	require.NoError(t, err)
	body, err := stream.DecryptBlob(blob, sdBlob.Key, sdBlob.BlobInfos[0].IV)
	require.Equal(t, stream.MaxBlobSize, len(body))
}

func TestGetReflectorVS3(t *testing.T) {
	uri := "known-size#0590f924bbee6627a2e79f7f2ff7dfb50bf2877c"
	// hash := "845cb8654e7e7fe2e83ce8059bfee600fb304299cfc402842f43999ea272526cbe0d852d155ce030a4a8e7f125907ec0"

	p := NewPlayer(nil)
	bStore := peer.NewStore(peer.StoreOpts{
		Address: config.GetRefractorAddress(),
		Timeout: time.Second * time.Duration(config.GetRefractorTimeout()),
	})
	s, err := p.ResolveStream(uri)
	require.NoError(t, err)

	sdBlob := stream.SDBlob{}
	blob, err := bStore.Get(s.Hash)
	err = sdBlob.FromBlob(blob)
	require.NoError(t, err)

	bi := sdBlob.BlobInfos[1]
	hash := hex.EncodeToString(bi.BlobHash)

	blob, err = bStore.Get(hash)
	require.NoError(t, err)

	r, err := http.Get("http://blobs.lbry.io/" + hash)
	require.NoError(t, err)
	defer r.Body.Close()
	require.Equal(t, http.StatusOK, r.StatusCode)
	s3blob, err := ioutil.ReadAll(r.Body)
	require.NoError(t, err)

	decBlob, err := stream.DecryptBlob(blob, sdBlob.Key, bi.IV)
	require.NoError(t, err)
	decS3blob, err := stream.DecryptBlob(s3blob, sdBlob.Key, bi.IV)
	require.NoError(t, err)

	assert.EqualValues(t, s3blob, blob, fmt.Sprintf("S3 blob len: %v, reflected blob len: %v", len(s3blob), len(blob)))
	assert.EqualValues(t, decBlob, decS3blob, fmt.Sprintf("S3 blob len: %v, reflected blob len: %v", len(decS3blob), len(decBlob)))
	// assert.Equal(t, "0fa7383b", hex.EncodeToString(decBlob[len(decBlob)-10:]))
	// assert.Equal(t, "0fa7383b", hex.EncodeToString(decS3blob[len(decS3blob)-10:]))
	exp := strings.ToLower("0FA7383B3760C5CE5DFC2F73BD5EE7")
	assert.Equal(t, exp, hex.EncodeToString(decBlob[:15]))
	assert.Equal(t, exp, hex.EncodeToString(decS3blob[:15]))
}
