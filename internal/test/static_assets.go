package test

import (
	"io"
	"net/http"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

var staticPath = "https://ik.imagekit.io/odystatic/"

func StaticAsset(t *testing.T, fileName string) string {
	t.Helper()
	t.Logf("getting static asset %s", staticPath+fileName)
	r, err := http.Get(staticPath + fileName)
	require.NoError(t, err)
	f, err := os.Create(path.Join(t.TempDir(), fileName))
	require.NoError(t, err)
	defer f.Close()
	_, err = io.Copy(f, r.Body)
	require.NoError(t, err)
	return f.Name()
}
