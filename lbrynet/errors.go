package lbrynet

import (
	"fmt"
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
