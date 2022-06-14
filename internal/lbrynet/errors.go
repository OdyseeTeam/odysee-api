package lbrynet

import (
	"fmt"
	"regexp"

	"github.com/OdyseeTeam/odysee-api/internal/errors"
)

type WalletError struct {
	UserID int
	Err    error
}

func (e WalletError) Error() string { return fmt.Sprintf("user %d: %s", e.UserID, e.Err.Error()) }
func (e WalletError) Unwrap() error { return e.Err }

var (
	ErrWalletNotFound      = errors.Base("wallet not found")
	ErrWalletExists        = errors.Base("wallet exists and is loaded")
	ErrWalletNeedsLoading  = errors.Base("wallet exists and needs to be loaded")
	ErrWalletNotLoaded     = errors.Base("wallet is not loaded")
	ErrWalletAlreadyLoaded = errors.Base("wallet is already loaded")

	// Workaround for non-existent SDK error codes
	reWalletNotFound      = regexp.MustCompile(`(?i)wallet at path .+ was not found`)
	reWalletExists        = regexp.MustCompile(`(?i)wallet at path .+ already exists and is loaded`)
	reWalletNeedsLoading  = regexp.MustCompile(`(?i)wallet at path .+ already exists, use 'wallet_add' to load wallet`)
	reWalletNotLoaded     = regexp.MustCompile(`(?i)couldn't find wallet:`)
	reWalletAlreadyLoaded = regexp.MustCompile(`(?i)wallet at path .+ is already loaded`)
)

// NewWalletError converts plain SDK error to the typed one
func NewWalletError(userID int, err error) error {
	switch {
	case reWalletNotFound.MatchString(err.Error()):
		return WalletError{UserID: userID, Err: ErrWalletNotFound}
	case reWalletExists.MatchString(err.Error()):
		return WalletError{UserID: userID, Err: ErrWalletExists}
	case reWalletNeedsLoading.MatchString(err.Error()):
		return WalletError{UserID: userID, Err: ErrWalletNeedsLoading}
	case reWalletNotLoaded.MatchString(err.Error()):
		return WalletError{UserID: userID, Err: ErrWalletNotLoaded}
	case reWalletAlreadyLoaded.MatchString(err.Error()):
		return WalletError{UserID: userID, Err: ErrWalletAlreadyLoaded}
	default:
		return WalletError{UserID: userID, Err: err}
	}
}
