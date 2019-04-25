package users

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Response is returned by API handlers
type Response struct {
	Status int
	Data   interface{}
	Error  error
}

// NewResponse is returned from handlers when client has provided invalid input
func NewResponse(d interface{}) Response {
	return Response{Status: http.StatusOK, Data: d}
}

// NewClientError is returned from handlers when client has provided invalid input
func NewClientError(e error) Response {
	return Response{Status: http.StatusBadRequest, Error: e}
}

// NewRemoteError is returned from handlers when there's been a remote service error
func NewRemoteError(e error) Response {
	return Response{Status: http.StatusServiceUnavailable, Error: e}
}

type handler func(r *http.Request) Response

var methods = map[string]handler{
	"new": HandleNew,
}

func HandleMethod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	response := methods[vars["method"]](r)
	w.WriteHeader(response.Status)
}

func HandleNew(r *http.Request) Response {
	return Response{Data: ""}
}
