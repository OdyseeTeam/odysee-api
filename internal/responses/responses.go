package responses

import (
	"net/http"
)

// this is the message to show when authentication info is required but was not provided in the request
// this is NOT the message for when auth info is provided but is not correct
const AuthRequiredErrorMessage = "authentication required"

// AddJSONContentType prepares HTTP response writer for JSON content-type.
func AddJSONContentType(w http.ResponseWriter) {
	w.Header().Add("content-type", "application/json; charset=utf-8")
}
