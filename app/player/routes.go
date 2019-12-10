package player

import "github.com/gorilla/mux"

func InstallRoutes(r *mux.Router) {
	playerHandler := NewRequestHandler(NewPlayer(&PlayerOpts{EnableLocalCache: true, EnablePrefetch: true}))
	playerRouter := r.Path("/content/claims/{uri}/{claim}/{filename}").Subrouter()
	playerRouter.HandleFunc("", playerHandler.Handle).Methods("GET")
	playerRouter.HandleFunc("", playerHandler.HandleOptions).Methods("OPTIONS")
}
