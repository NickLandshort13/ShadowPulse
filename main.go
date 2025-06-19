package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/urfave/cli/v2"
	"golang.org/x/net/proxy"
)

var (
	debugMode = true
	proxyURLs = map[string]string{
		"http":   "https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt",
		"socks5": "https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks5.txt",
	}
)

type Proxy struct {
	IP       string
	Port     string
	Protocol string
}

func debugLog(format string, args ...interface{}) {
	if debugMode {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func main() {
	app := &cli.App{
		Name:  "proxyctl",
		Usage: "Multi-protocol proxy chain manager",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "Enable debug mode",
				Destination: &debugMode,
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "update",
				Usage: "Update proxy list",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "type",
						Value: "http",
						Usage: "Proxy type (http, socks5)",
					},
				},
				Action: updateProxies,
			},
			{
				Name:  "validate",
				Usage: "Validate proxies",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "type",
						Value: "http",
						Usage: "Proxy type (http, socks5)",
					},
				},
				Action: validateProxies,
			},
			{
				Name:  "run",
				Usage: "Run local proxy server",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "chain",
						Value: 3,
						Usage: "Proxy chain length",
					},
					&cli.IntFlag{
						Name:  "port",
						Value: 8080,
						Usage: "Local proxy port",
					},
					&cli.StringFlag{
						Name:  "type",
						Value: "http",
						Usage: "Proxy type (http, socks5)",
					},
				},
				Action: runProxy,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func updateProxies(c *cli.Context) error {
	proxyType := c.String("type")
	url, ok := proxyURLs[proxyType]
	if !ok {
		return fmt.Errorf("unsupported proxy type: %s", proxyType)
	}

	debugLog("Fetching %s proxies from: %s", proxyType, url)
	proxies, err := fetchProxiesFromURL(url, proxyType)
	if err != nil {
		return fmt.Errorf("failed to fetch proxies: %v", err)
	}

	filename := getProxyFilename(proxyType)
	if err := saveProxiesToFile(proxies, filename); err != nil {
		return fmt.Errorf("failed to save proxies: %v", err)
	}

	debugLog("Saved %d %s proxies to %s", len(proxies), proxyType, filename)
	fmt.Printf("Successfully updated %d %s proxies\n", len(proxies), proxyType)
	return nil
}

func getProxyFilename(proxyType string) string {
	return fmt.Sprintf("proxies_%s.txt", proxyType)
}

func getValidProxyFilename(proxyType string) string {
	return fmt.Sprintf("valid_proxies_%s.txt", proxyType)
}

func fetchProxiesFromURL(url, proxyType string) ([]Proxy, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var proxies []Proxy
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) >= 2 {
			proxies = append(proxies, Proxy{
				IP:       parts[0],
				Port:     parts[1],
				Protocol: proxyType,
			})
		}
	}
	return proxies, scanner.Err()
}

func saveProxiesToFile(proxies []Proxy, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, p := range proxies {
		if _, err := file.WriteString(fmt.Sprintf("%s:%s:%s\n", p.IP, p.Port, p.Protocol)); err != nil {
			return err
		}
	}
	return nil
}

func validateProxies(c *cli.Context) error {
	proxyType := c.String("type")
	filename := getProxyFilename(proxyType)

	debugLog("Loading proxies from: %s", filename)
	proxies, err := loadProxiesFromFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("proxy file not found, please run 'update' command first")
		}
		return fmt.Errorf("failed to load proxies: %v", err)
	}

	totalProxies := len(proxies)
	debugLog("Validating %d %s proxies with goroutines...", totalProxies, proxyType)

	validProxies := make([]Proxy, 0)
	var mutex sync.Mutex
	var wg sync.WaitGroup

	workers := 100
	proxyChan := make(chan Proxy, workers)
	progressChan := make(chan int, totalProxies)

	var (
		checkedCount int32
		validCount   int32
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range proxyChan {
				isValid := validateProxy(p, 8*time.Second)
				if isValid {
					mutex.Lock()
					validProxies = append(validProxies, p)
					mutex.Unlock()
					atomic.AddInt32(&validCount, 1)
					debugLog("Proxy %s:%s is valid", p.IP, p.Port)
				} else {
					debugLog("Proxy %s:%s failed validation", p.IP, p.Port)
				}
				atomic.AddInt32(&checkedCount, 1)
				progressChan <- 1
			}
		}()
	}

	go func() {
		for _, p := range proxies {
			proxyChan <- p
		}
		close(proxyChan)
	}()

	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				checked := atomic.LoadInt32(&checkedCount)
				valid := atomic.LoadInt32(&validCount)
				percent := float64(checked) / float64(totalProxies) * 100
				fmt.Printf("\rProgress: %.2f%% | Checked: %d/%d | Valid: %d", percent, checked, totalProxies, valid)
			}
		}
	}()

	wg.Wait()
	close(progressChan)
	done <- true
	fmt.Println()

	validFilename := getValidProxyFilename(proxyType)
	if err := saveProxiesToFile(validProxies, validFilename); err != nil {
		return fmt.Errorf("failed to save valid proxies: %v", err)
	}

	debugLog("Saved %d valid %s proxies to %s", len(validProxies), proxyType, validFilename)
	fmt.Printf("\nValidation complete. Valid: %d/%d (%.2f%%)\n",
		len(validProxies),
		totalProxies,
		float64(len(validProxies))/float64(totalProxies)*100)

	return nil
}

func loadProxiesFromFile(filename string) ([]Proxy, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var proxies []Proxy
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) >= 2 {
			protocol := "http"
			if len(parts) >= 3 {
				protocol = parts[2]
			}
			proxies = append(proxies, Proxy{
				IP:       parts[0],
				Port:     parts[1],
				Protocol: protocol,
			})
		}
	}
	return proxies, scanner.Err()
}

func validateProxy(p Proxy, timeout time.Duration) bool {
	debugLog("Validating proxy %s://%s:%s", p.Protocol, p.IP, p.Port)

	var client *http.Client
	switch p.Protocol {
	case "http", "https":
		proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%s", p.IP, p.Port))
		if err != nil {
			debugLog("Invalid proxy URL: %v", err)
			return false
		}
		client = &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
			Timeout:   timeout,
		}
	case "socks5":
		dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%s", p.IP, p.Port), nil, proxy.Direct)
		if err != nil {
			debugLog("SOCKS proxy error: %v", err)
			return false
		}
		client = &http.Client{
			Transport: &http.Transport{Dial: dialer.Dial},
			Timeout:   timeout,
		}
	default:
		debugLog("Unsupported protocol: %s", p.Protocol)
		return false
	}

	req, err := http.NewRequest("GET", "https://httpbin.org/ip", nil)
	if err != nil {
		debugLog("Request creation failed: %v", err)
		return false
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		debugLog("Request failed after %v: %v", duration, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		debugLog("Non-200 status: %d", resp.StatusCode)
		return false
	}

	var result struct{ Origin string }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		debugLog("Failed to decode response: %v", err)
		return false
	}

	debugLog("Proxy %s:%s works! Your IP: %s (response time: %v)", p.IP, p.Port, result.Origin, duration)
	return true
}

func runProxy(c *cli.Context) error {
	chainSize := c.Int("chain")
	port := c.Int("port")
	proxyType := c.String("type")
	filename := getValidProxyFilename(proxyType)

	debugLog("Loading valid proxies from: %s", filename)
	proxies, err := loadProxiesFromFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("valid proxies file not found, please run 'update' and 'validate' commands first")
		}
		return fmt.Errorf("failed to load proxies: %v", err)
	}

	if len(proxies) == 0 {
		return fmt.Errorf("no valid proxies available")
	}

	for {
		debugLog("Building proxy chain (size: %d)", chainSize)
		dialer, err := buildProxyChain(proxies, chainSize, proxyType)
		if err != nil {
			debugLog("Failed to build chain: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if testProxyChain(dialer) {
			debugLog("Proxy chain is working, starting server on :%d", port)
			if err := startProxyServer(port, dialer); err != nil {
				debugLog("Server error: %v", err)
			}
			time.Sleep(5 * time.Second)
		} else {
			debugLog("Proxy chain failed, retrying...")
			time.Sleep(5 * time.Second)
		}
	}
}

func buildProxyChain(proxies []Proxy, chainSize int, proxyType string) (proxy.Dialer, error) {
	if len(proxies) == 0 {
		return nil, fmt.Errorf("no proxies available")
	}

	if chainSize > len(proxies) {
		chainSize = len(proxies)
	}

	rand.Shuffle(len(proxies), func(i, j int) {
		proxies[i], proxies[j] = proxies[j], proxies[i]
	})

	var dialer proxy.Dialer = proxy.Direct

	for i := 0; i < chainSize; i++ {
		p := proxies[i]
		debugLog("Adding to chain %d: %s://%s:%s", i+1, p.Protocol, p.IP, p.Port)

		switch p.Protocol {
		case "socks5":
			newDialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%s", p.IP, p.Port), nil, dialer)
			if err != nil {
				return nil, fmt.Errorf("SOCKS5 error: %v", err)
			}
			dialer = newDialer

		case "http":
			proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%s", p.IP, p.Port))
			if err != nil {
				return nil, fmt.Errorf("invalid HTTP proxy URL: %v", err)
			}
			dialer = &httpProxyDialer{
				proxy:   proxyURL,
				forward: dialer,
			}

		default:
			return nil, fmt.Errorf("unsupported protocol: %s", p.Protocol)
		}
	}

	return dialer, nil
}

type httpProxyDialer struct {
	proxy   *url.URL
	forward proxy.Dialer
}

func (h *httpProxyDialer) Dial(network, addr string) (net.Conn, error) {
	conn, err := h.forward.Dial(network, h.proxy.Host)
	if err != nil {
		return nil, err
	}

	req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", addr, addr)
	if _, err = conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}

	buf := make([]byte, 1024)
	if _, err = conn.Read(buf); err != nil {
		conn.Close()
		return nil, err
	}

	if !strings.Contains(string(buf), "200") {
		conn.Close()
		return nil, fmt.Errorf("proxy refused connection")
	}

	return conn, nil
}

func testProxyChain(dialer proxy.Dialer) bool {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	debugLog("Testing proxy chain...")
	req, err := http.NewRequest("GET", "https://httpbin.org/ip", nil)
	if err != nil {
		debugLog("Test request creation failed: %v", err)
		return false
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		debugLog("Proxy test failed after %v: %v", duration, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		debugLog("Non-200 status: %d", resp.StatusCode)
		return false
	}

	var result struct{ Origin string }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		debugLog("Failed to decode response: %v", err)
		return false
	}

	debugLog("Proxy chain test successful! Your IP: %s (response time: %v)", result.Origin, duration)
	return true
}

func startProxyServer(port int, dialer proxy.Dialer) error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "CONNECT" {
			targetConn, err := dialer.Dial("tcp", r.Host)
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}

			hijacker, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
				return
			}

			clientConn, _, err := hijacker.Hijack()
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}

			clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

			go io.Copy(targetConn, clientConn)
			io.Copy(clientConn, targetConn)
			return
		}

		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}

		client := &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}

		r.RequestURI = ""
		r.URL.Scheme = "http"
		if r.URL.Port() == "443" {
			r.URL.Scheme = "https"
		}

		resp, err := client.Do(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}

	return server.ListenAndServe()
}
