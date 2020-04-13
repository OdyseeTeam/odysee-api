package responses

import (
	"net/http"
)

// AddJSONContentType prepares HTTP response writer for JSON content-type.
func AddJSONContentType(w http.ResponseWriter) {
	w.Header().Add("content-type", "application/json; charset=utf-8")
}
