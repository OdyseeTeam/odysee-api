package users

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lbryio/lbry.go/extras/api"
	"github.com/lbryio/lbryweb.go/lbryinc"
	"github.com/lbryio/lbryweb.go/lbrynet"
)

type handler func(r *http.Request) api.Response

var methods = map[string]handler{
	"new": HandleNew,
}

func HandleMethod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	response := methods[vars["method"]](r)
	w.WriteHeader(http.StatusOK)
}

func HandleNew(r *http.Request) api.Response {
	response, err := lbryinc.Call("user", "new", r.Body)
	if err != nil {
		panic(fmt.Errorf("error calling internal-apis: %v", err))
	}
	if response.Error != nil {
		panic(response.Error)
	}
	lbrynet.Client.Call()
	CreateRecord(response.Data["auth_token"])
	return
}
