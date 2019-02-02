package updater

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLatestRelease(t *testing.T) {
	GetLatestRelease("sayplastic/lbryweb-js", "/tmp/_TestGetLatestRelease")

	_, err := os.Stat("/tmp/_TestGetLatestRelease/app/bundle.js")
	assert.Nil(t, err)

	err = os.RemoveAll("/tmp/_TestGetLatestRelease")
	if err != nil {
		panic(err)
	}
}
