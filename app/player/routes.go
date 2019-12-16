package player

import (
	"net/http"

	"github.com/gorilla/mux"
)

func InstallRoutes(r *mux.Router) {
	playerHandler := NewRequestHandler(NewPlayer(&Opts{EnableLocalCache: true, EnablePrefetch: true}))
	playerRouter := r.Path("/content/claims/{uri}/{claim}/{filename}").Subrouter()
	playerRouter.HandleFunc("", playerHandler.Handle).Methods(http.MethodGet)
	playerRouter.HandleFunc("", playerHandler.HandleHead).Methods(http.MethodHead)
}
