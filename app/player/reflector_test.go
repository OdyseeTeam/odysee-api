package player

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/lbryio/lbry.go/v2/stream"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/reflector.go/peer"
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
