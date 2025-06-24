package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Proxy struct {
	IP      string  `json:"ip"`
	Port    int     `json:"port"`
	Latency int     `json:"latency"`
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

func FetchProxies() ([]Proxy, error) {
	resp, err := http.Get("https://proxylist.geonode.com/api/proxy-list?limit=50&page=1")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data []struct {
			IP      string  `json:"ip"`
			Port    string  `json:"port"`
			Latency float64 `json:"latency"`
			Country string  `json:"country"`
			Geo     struct {
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
			} `json:"geo"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	var proxies []Proxy
	for _, p := range response.Data {
		port := 0
		if _, err := fmt.Sscanf(p.Port, "%d", &port); err == nil {
			proxies = append(proxies, Proxy{
				IP:      p.IP,
				Port:    port,
				Latency: int(p.Latency),
				Country: p.Country,
				Lat:     p.Geo.Lat,
				Lng:     p.Geo.Lon,
			})
		}
	}

	return proxies, nil
}
