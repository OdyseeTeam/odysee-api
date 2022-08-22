package lbrynet

import (
	"testing"

	"github.com/OdyseeTeam/odysee-api/internal/errors"
	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/stretchr/testify/assert"
)

func TestWalletAlreadyLoaded(t *testing.T) {
	err := ljsonrpc.Error{
		Code:    123,
		Name:    ljsonrpc.ErrorWalletAlreadyLoaded,
		Message: "Wallet 123.wallet is already loaded",
	}
	assert.True(t, errors.Is(NewWalletError(err), ErrWalletAlreadyLoaded))
}
