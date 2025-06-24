package api

import (
	"encoding/json"
	"net/http"

	"github.com/NickLandshort13/ShadowPulse/internal/proxy"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func NewRouter() http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/api/proxies", func(w http.ResponseWriter, r *http.Request) {
		proxies, err := proxy.FetchProxies()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(proxies)
	}).Methods("GET", "OPTIONS")

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
	})

	return c.Handler(r)
}
