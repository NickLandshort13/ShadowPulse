package main

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/NickLandshort13/ShadowPulse/internal/api"
	"github.com/oschwald/geoip2-golang"
)

func main() {
	dbPath := filepath.Join("dbs", "GeoLite2-City.mmdb")
	db, err := geoip2.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open GeoIP database: %v", err)
	}
	defer db.Close()

	router := api.NewRouter(db)
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
