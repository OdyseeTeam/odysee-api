package lbrynet

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWalletAlreadyLoaded(t *testing.T) {
	origErr := fmt.Errorf("Wallet at path /tmp/123 is already loaded")
	walletErr := &WalletAlreadyLoaded{}
	err := NewWalletError(123, origErr)

	assert.True(t, errors.As(err, walletErr))
	assert.Equal(t, 123, walletErr.UID)
}
