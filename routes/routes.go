package routes

import "github.com/gorilla/mux"

// InstallRoutes sets up global API handlers
func InstallRoutes(r *mux.Router) {
	r.HandleFunc("/", Index)
	r.HandleFunc("/api/proxy", Proxy)
	r.HandleFunc("/api/proxy/", Proxy)
	r.HandleFunc("/content/claims/{uri}/{claim}/{filename}", ContentByClaimsURI)
	r.HandleFunc("/content/url", ContentByURL)
}
