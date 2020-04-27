package errors

import (
	base "errors"
	"testing"

	pkg "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestErr_MultipleLayersOfWrapping(t *testing.T) {
	orig := base.New("the base error")
	pkg1 := pkg.Wrap(orig, "wrapped pkg 1")
	our1 := Err(pkg1)
	pkg2 := pkg.Wrap(our1, "wrapped pkg 2")
	our2 := Err(pkg2)
	assert.True(t, base.Is(our1, orig))
	assert.True(t, base.Is(our2, orig))
	assert.True(t, base.Is(pkg2, orig))
	assert.True(t, base.Is(our2, pkg1))
	assert.True(t, base.Is(our2, our1))
}
