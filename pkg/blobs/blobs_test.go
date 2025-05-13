package blobs

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/lbryio/lbry.go/v3/stream"
	"github.com/spf13/viper"

	"github.com/stretchr/testify/require"
)

func TestSplit(t *testing.T) {
	filePath := test.StaticAsset(t, "doc.pdf")

	s := NewSource(filePath, t.TempDir(), "doc.pdf")
	pbs, err := s.Split()
	require.NoError(t, err)
	require.Equal(t, "doc.pdf", pbs.GetSource().Name)

	require.NoError(t, err)
	stream := make(stream.Stream, len(s.blobsManifest))
	for i, b := range s.blobsManifest {
		data, err := os.ReadFile(path.Join(s.finalPath, b))
		require.NoError(t, err)
		stream[i] = data
	}

	result, err := stream.Decode()

	require.NoError(t, err)
	original, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, original, result)
}

func TestConfig(t *testing.T) {
	require := require.New(t)

	v := viper.New()
	v.SetConfigType("yaml")

	cfgPath, _ := filepath.Abs("./testdata/config.yml")
	f, err := os.Open(cfgPath)
	require.NoError(err)
	err = v.ReadConfig(f)
	require.NoError(err)

	stores, err := CreateStoresFromConfig(v, "ReflectorStorage.Destinations")
	require.NoError(err)
	require.Len(stores, 2)
	require.Equal("s3-another", stores[0].Name())
	require.Equal("s3-wasabi", stores[1].Name())
	require.True(false)
}
