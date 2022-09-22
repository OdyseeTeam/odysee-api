package blobs

import (
	"encoding/hex"
	"io/ioutil"
	"path"
	"strings"
	"testing"

	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/lbryio/lbry.go/v3/stream"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplit(t *testing.T) {
	filePath := test.StaticAsset(t, "doc.pdf")

	s, err := NewSource(filePath, t.TempDir())
	require.NoError(t, err)
	pbs, err := s.Split()
	require.NoError(t, err)
	require.Equal(t, "doc.pdf", pbs.GetSource().Name)

	require.NoError(t, err)
	stream := make(stream.Stream, len(s.blobsManifest))
	for i, b := range s.blobsManifest {
		data, err := ioutil.ReadFile(path.Join(s.finalPath, b))
		require.NoError(t, err)
		stream[i] = data
	}

	result, err := stream.Decode()

	require.NoError(t, err)
	original, err := ioutil.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, original, result)
	assert.True(t, strings.HasSuffix(s.finalPath, hex.EncodeToString(s.Stream().GetSource().SdHash)))
}