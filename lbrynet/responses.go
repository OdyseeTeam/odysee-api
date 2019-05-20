package lbrynet

import (
	"fmt"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
)

type AccountNotFound struct {
	Email string
}

type AccountConflict struct {
	Email string
}

func (e AccountNotFound) Error() string {
	return fmt.Sprintf("couldn't find account for email %s", e.Email)
}

func (e AccountConflict) Error() string {
	return fmt.Sprintf("account for email %s already exists", e.Email)
}

// SingleAccountListResponse is a singular account_list response structure for when account_list is called with account_id as an argument
// TODO: move this to lbry.go/extras/jsonrpc
type SingleAccountListResponse struct {
	ljsonrpc.Account
}
