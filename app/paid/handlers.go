package paid

import (
	"net/http"
)

// HandlePublicKeyRequest delivers marshaled pubkey to remote agents for media token verification
func HandlePublicKeyRequest(w http.ResponseWriter, r *http.Request) {
	w.Write(km.PublicKeyBytes())
}
