package proxy

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/oschwald/geoip2-golang"
)

type Proxy struct {
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	Latency   int       `json:"latency"`
	Country   string    `json:"country"`
	City      string    `json:"city"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Type      string    `json:"type"`
	LastCheck time.Time `json:"lastCheck"`
}

func FetchProxies(db *geoip2.Reader) ([]Proxy, error) {
	resp, err := http.Get("https://proxylist.geonode.com/api/proxy-list?limit=500&page=1")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Data []struct {
			IP        string   `json:"ip"`
			Port      string   `json:"port"`
			Latency   float64  `json:"latency"`
			Protocols []string `json:"protocols"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	var proxies []Proxy
	for _, p := range response.Data {
		port := 0
		if _, err := fmt.Sscanf(p.Port, "%d", &port); err != nil {
			continue
		}

		ip := net.ParseIP(p.IP)
		if ip == nil {
			continue
		}

		record, err := db.City(ip)
		if err != nil {
			continue
		}

		proxyType := "http"
		if len(p.Protocols) > 0 {
			proxyType = p.Protocols[0]
		}

		cityName := "N/A"
		if len(record.City.Names) > 0 {
			cityName = record.City.Names["en"]
		}

		proxies = append(proxies, Proxy{
			IP:        p.IP,
			Port:      port,
			Latency:   int(p.Latency),
			Country:   record.Country.Names["en"],
			City:      cityName,
			Lat:       record.Location.Latitude,
			Lng:       record.Location.Longitude,
			Type:      proxyType,
			LastCheck: time.Now(),
		})
	}

	return proxies, nil
}
