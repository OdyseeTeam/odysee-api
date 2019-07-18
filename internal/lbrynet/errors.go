package lbrynet

import (
	"fmt"
)

type AccountNotFound struct {
	UID int
}

type AccountConflict struct {
	UID int
}

func (e AccountNotFound) Error() string {
	return fmt.Sprintf("couldn't find account for %v in lbrynet", e.UID)
}

func (e AccountConflict) Error() string {
	return fmt.Sprintf("account for %v already registered with lbrynet", e.UID)
}
