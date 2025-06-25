package api

import (
	"encoding/json"
	"net/http"

	"github.com/NickLandshort13/ShadowPulse/internal/proxy"
	"github.com/gorilla/mux"
	"github.com/oschwald/geoip2-golang"
	"github.com/rs/cors"
)

func NewRouter(db *geoip2.Reader) http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/api/proxies", func(w http.ResponseWriter, r *http.Request) {
		proxies, err := proxy.FetchProxies(db)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(proxies)
	}).Methods("GET")

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET"},
	})

	return c.Handler(r)
}
