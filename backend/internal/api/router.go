package api

import (
	"encoding/json"
	"net/http"

	"github.com/NickLandshort13/ShadowPulse/internal/proxy"
	"github.com/gorilla/mux"
)

type ProxyResponse struct {
	IP      string  `json:"ip"`
	Latency int     `json:"latency"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

func NewRouter() *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/api/proxies", func(w http.ResponseWriter, r *http.Request) {
		proxies := proxy.GetActiveProxies()
		json.NewEncoder(w).Encode(proxies)
	}).Methods("GET")

	return r
}
