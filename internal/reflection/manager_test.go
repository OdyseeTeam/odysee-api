package reflection

import (
	"os"
	"testing"

	"github.com/lbryio/lbrytv/config"
	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	p := NewManager(os.TempDir(), config.GetReflectorAddress())
	p.Initialize()
	assert.True(t, p.IsInitialized())
	assert.NotNil(t, p.uploader)

	p = NewManager(os.TempDir(), "")
	p.Initialize()
	assert.False(t, p.IsInitialized())

	p = NewManager("/random_nonexistant_dir/", config.GetReflectorAddress())
	p.Initialize()
	assert.False(t, p.IsInitialized())
}
