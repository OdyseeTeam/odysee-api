package blobs

import (
	"encoding/hex"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lbryio/lbry.go/v3/stream"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplit(t *testing.T) {
	absPath, _ := filepath.Abs("./testdata/assembly.pdf")

	s, err := NewSource(absPath, t.TempDir())
	require.NoError(t, err)
	pbs, err := s.Split()
	require.NoError(t, err)
	require.Equal(t, "assembly.pdf", pbs.GetSource().Name)

	require.NoError(t, err)
	stream := make(stream.Stream, len(s.blobsManifest))
	for i, b := range s.blobsManifest {
		data, err := ioutil.ReadFile(path.Join(s.finalPath, b))
		require.NoError(t, err)
		stream[i] = data
	}

	result, err := stream.Decode()

	require.NoError(t, err)
	original, err := ioutil.ReadFile(absPath)
	require.NoError(t, err)
	require.Equal(t, original, result)
	assert.True(t, strings.HasSuffix(s.finalPath, hex.EncodeToString(s.Stream().GetSource().SdHash)))
}
