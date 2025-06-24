package proxy

type Proxy struct {
	IP      string
	Latency int
	Country string
}

var activeProxies = []Proxy{
	{"192.168.1.1:8080", 120, "US"},
	{"193.32.1.5:3128", 80, "DE"},
}

func GetActiveProxies() []Proxy {
	return activeProxies
}
