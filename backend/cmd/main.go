package main

import (
	"log"
	"net/http"

	"github.com/NickLandshort13/ShadowPulse/internal/api"
)

func main() {
	router := api.NewRouter()
	log.Println("ShadowPulse запущен на :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
