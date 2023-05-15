package keybox

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
)

// PublicKeyHandler returns a HTTP handler that delivers marshaled public key over HTTP request.
func PublicKeyHandler(pk crypto.PublicKey) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pkb, err := x509.MarshalPKIXPublicKey(pk)
		if err != nil {
			http.Error(w, fmt.Sprintf("error marshaling pubkey: %s", err), http.StatusInternalServerError)
			return
		}

		pemData := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: pkb,
		})

		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Write(pemData)
	}
}
