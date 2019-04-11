package accounts

import (
	"net/http"
)

// New is called by UI for every new visitor.
// Creates a new account with the SDK and forwards client payload to internal-apis/user/new
func New(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}
