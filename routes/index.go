package routes

import (
	"net/http"

	"github.com/lbryio/lbry.go/api"
)

func Index(r *http.Request) api.Response {
	return api.Response{Data: "lbryweb api response goes here"}
}
