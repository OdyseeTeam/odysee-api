package lbrynet

import (
	"github.com/OdyseeTeam/odysee-api/internal/errors"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
)

type WalletError struct {
	// UserID int
	Err error
}

func (e WalletError) Error() string { return e.Err.Error() }
func (e WalletError) Unwrap() error { return e.Err }

var (
	ErrWalletNotFound      = errors.Base("wallet not found")
	ErrWalletExists        = errors.Base("wallet exists and is loaded")
	ErrWalletNeedsLoading  = errors.Base("wallet exists and needs to be loaded")
	ErrWalletNotLoaded     = errors.Base("wallet is not loaded")
	ErrWalletAlreadyLoaded = errors.Base("wallet is already loaded")
)

// NewWalletError converts plain SDK error to the typed one
func NewWalletError(err error) error {
	var derr ljsonrpc.Error
	var ok bool
	if derr, ok = err.(ljsonrpc.Error); !ok {
		return WalletError{Err: err}
	}
	switch derr.Name {
	case ljsonrpc.ErrorWalletNotFound:
		return WalletError{Err: ErrWalletNotFound}
	case ljsonrpc.ErrorWalletAlreadyExists:
		return WalletError{Err: ErrWalletExists}
	case ljsonrpc.ErrorWalletNotLoaded:
		return WalletError{Err: ErrWalletNotLoaded}
	case ljsonrpc.ErrorWalletAlreadyLoaded:
		return WalletError{Err: ErrWalletAlreadyLoaded}
	default:
		return WalletError{Err: err}
	}
}
