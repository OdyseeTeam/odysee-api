package lbrynet

import (
	"fmt"
	"regexp"
)

type AccountNotFound struct {
	UID int
	Err error
}

type AccountConflict struct {
	UID int
	Err error
}

type WalletError struct {
	error
	UID int
	Err error
}

type WalletExists struct {
	WalletError
}

type WalletNeedsLoading struct {
	WalletError
}

type WalletAlreadyLoaded struct {
	WalletError
}

type WalletNotFound struct {
	WalletError
}

type WalletNotLoaded struct {
	WalletError
}

func (e AccountNotFound) Error() string {
	return fmt.Sprintf("couldn't find account for %v in lbrynet", e.UID)
}

func (e AccountConflict) Error() string {
	return fmt.Sprintf("account for %v already registered with lbrynet", e.UID)
}

// Workaround for non-existent SDK error codes
var reWalletExists = regexp.MustCompile(`Wallet at path .+ already exists and is loaded`)
var reWalletNeedsLoading = regexp.MustCompile(`Wallet at path .+ already exists, use 'wallet_add' to load wallet`)
var reWalletAlreadyLoaded = regexp.MustCompile(`Wallet at path .+ is already loaded`)
var reWalletNotFound = regexp.MustCompile(`Wallet at path .+ was not found`)
var reWalletNotLoaded = regexp.MustCompile(`Couldn't find wallet:`)

// NewWalletError converts plain SDK error to the typed one
func NewWalletError(uid int, err error) error {
	wErr := WalletError{UID: uid, Err: err}

	switch {
	case reWalletExists.MatchString(err.Error()):
		return WalletExists{wErr}
	case reWalletNeedsLoading.MatchString(err.Error()):
		return WalletNeedsLoading{wErr}
	case reWalletAlreadyLoaded.MatchString(err.Error()):
		return WalletAlreadyLoaded{wErr}
	case reWalletNotFound.MatchString(err.Error()):
		return WalletNotFound{wErr}
	case reWalletNotLoaded.MatchString(err.Error()):
		return WalletNotLoaded{wErr}
	default:
		return wErr
	}
}

func (e WalletError) Unwrap() error {
	return e.Err
}

func (e WalletError) Error() string {
	return fmt.Sprintf("unknown wallet error: %v", e.Unwrap())
}

func (e WalletExists) Error() string {
	return "wallet is already loaded"
}

func (e WalletNeedsLoading) Error() string {
	return "wallet already exists but is not loaded"
}

func (e WalletAlreadyLoaded) Error() string {
	return "wallet is already loaded"
}

func (e WalletNotLoaded) Error() string {
	return "wallet not found"
}
