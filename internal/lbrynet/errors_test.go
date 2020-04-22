package lbrynet

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWalletAlreadyLoaded(t *testing.T) {
	walletErr := &WalletError{}
	err := NewWalletError(123, errors.New("Wallet at path /tmp/123 is already loaded"))
	assert.True(t, errors.Is(err, ErrWalletAlreadyLoaded))
	assert.True(t, errors.As(err, walletErr))
	assert.Equal(t, 123, walletErr.UserID)
}
