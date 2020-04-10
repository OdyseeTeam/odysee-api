package player

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/mux"
)

type rangeHeader struct {
	start, end, knownLen int
}

func makeRequest(router *mux.Router, method, uri string, rng *rangeHeader) *http.Response {
	if router == nil {
		router = mux.NewRouter()
		InstallRoutes(router)
	}

	r, _ := http.NewRequest(method, uri, nil)
	if rng != nil {
		if rng.start == 0 {
			r.Header.Add("Range", fmt.Sprintf("bytes=0-%v", rng.end))
		} else if rng.end == 0 {
			r.Header.Add("Range", fmt.Sprintf("bytes=%v-", rng.start))
		} else {
			r.Header.Add("Range", fmt.Sprintf("bytes=%v-%v", rng.start, rng.end))
		}
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, r)
	return rr.Result()
}
