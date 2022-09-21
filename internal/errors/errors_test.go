package errors

import (
	base "errors"
	"fmt"
	"testing"

	pkg "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestRecover(t *testing.T) {
	var err error
	require.NotPanics(t, func() {
		err = func() (e error) {
			defer Recover(&e)
			doPanic()
			return nil
		}()
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "who shall dwell in these worlds")

	withTrace, ok := err.(*traced)
	assert.True(t, ok)

	stackFrames := withTrace.StackFrames()
	fmt.Println(stackFrames)
	assert.Equal(t, "doYetDeeperPanic", stackFrames[0].Name)
	assert.Equal(t, "doDeeperPanic", stackFrames[1].Name)
	assert.Equal(t, "doPanic", stackFrames[2].Name)

	traceStr := Trace(err)
	assert.Contains(t, traceStr, "who shall dwell in these worlds")
}

func doPanic() {
	doDeeperPanic()
}

func doDeeperPanic() {
	doYetDeeperPanic()
}

func doYetDeeperPanic() {
	panic("But who shall dwell in these worlds if they be inhabited?… Are we or they Lords of the World?… And how are all things made for man?")
}

func TestErr_NilPointer(t *testing.T) {
	e := Err(nil)
	assert.NotPanics(t, func() {
		Unwrap(e)
	})
}
