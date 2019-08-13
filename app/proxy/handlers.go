package proxy

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/config"
)

type RequestHandler struct {
	*Service
}

func (rh *RequestHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var accountID string

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Body == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("empty request body"))
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Panicf("error: %v", err.Error())
	}

	c := rh.Service.NewCaller()

	if config.AccountsEnabled() {
		accountID, err = users.GetAccountIDFromRequest(r)

		// TODO: Refactor response creation out of this function
		if err != nil {
			response, _ := json.Marshal(NewErrorResponse(err.Error(), ErrAuthFailed))
			w.WriteHeader(http.StatusForbidden)
			w.Write(response)
			return
		}
		c.SetAccountID(accountID)
	}

	rawCallReponse := c.Call(body)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(rawCallReponse)
}
