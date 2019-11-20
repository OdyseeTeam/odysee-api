package wallet

import "fmt"

const walletNameTemplate string = "lbrytv-id.%v.wallet"

// MakeID formats user ID to use as an LbrynetServer wallet ID.
func MakeID(uid int) string {
	return fmt.Sprintf(walletNameTemplate, uid)
}
