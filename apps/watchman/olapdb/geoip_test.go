package olapdb

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getArea(t *testing.T) {
	p, _ := filepath.Abs(filepath.Join("./testdata", "GeoIP2-City-Test.mmdb"))
	err := OpenGeoDB(p)
	require.NoError(t, err)
	assert.Equal(t, "gb", getArea("81.2.69.142"))
	assert.Equal(t, "", getArea("2001:41d0:303:df3e::"))
}
