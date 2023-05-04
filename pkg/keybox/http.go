package keybox

import (
	"crypto/x509"
	"net/http"
)

// PublicKeyHandler returns a HTTP handler that delivers marshaled public key over HTTP request.
func PublicKeyHandler(f Keyfob) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pkb, err := x509.MarshalPKIXPublicKey(f.publicKey)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write(pkb)
	}
}
