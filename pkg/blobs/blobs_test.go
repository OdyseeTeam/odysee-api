package blobs

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/lbryio/lbry.go/v3/stream"

	"github.com/stretchr/testify/require"
)

func TestSplit(t *testing.T) {
	filePath := test.StaticAsset(t, "doc.pdf")

	s := NewSource(filePath, t.TempDir())
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
}
