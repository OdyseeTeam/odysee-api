package responses

import (
	"encoding/json"
	"net/http"

	"github.com/ybbus/jsonrpc"
)

// PrepareJSONWriter prepares HTTP response writer for JSON content-type.
func PrepareJSONWriter(w http.ResponseWriter) {
	w.Header().Add("content-type", "application/json; charset=utf-8")
}

// JSON is a shorthand for serializing provided structure and writing it into the provided HTTP writer as JSON.
func JSON(w http.ResponseWriter, v interface{}) {
	r, _ := json.Marshal(v)
	PrepareJSONWriter(w)
	w.Write(r)
}

// JSONRPCError is a shorthand for creating an RPCResponse instance with specified error message and code.
func JSONRPCError(w http.ResponseWriter, message string, code int) {
	JSON(w, NewJSONRPCError(message, code))
}

func NewJSONRPCError(message string, code int) *jsonrpc.RPCResponse {
	return &jsonrpc.RPCResponse{JSONRPC: "2.0", Error: &jsonrpc.RPCError{
		Code:    code,
		Message: message,
	}}
}
