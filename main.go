package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/urfave/cli/v2"
)

var (
	debugMode      = true
	proxyChainSize = 3
)

func debugLog(format string, args ...interface{}) {
	if debugMode {
		log.Printf("[DEBUG] "+format, args...)
	}
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.212 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/535.11 (KHTML, like Gecko) Chrome/90.0.4430.212 Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/90.0.4430.212 Mobile/15E148 Safari/605.1",
}

type Proxy struct {
	IP       string
	Port     string
	Protocol string
}

func getRandomUserAgent() string {
	rand.Seed(time.Now().UnixNano())
	return userAgents[rand.Intn(len(userAgents))]
}

func fetchProxiesFromURL(url string) ([]Proxy, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var proxies []Proxy
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) >= 2 {
			protocol := "http"
			if len(parts) > 2 {
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

func saveProxiesToFile(proxies []Proxy, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, p := range proxies {
		_, err := file.WriteString(fmt.Sprintf("%s:%s:%s\n", p.IP, p.Port, p.Protocol))
		if err != nil {
			return err
		}
	}
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
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) >= 2 {
			protocol := "http"
			if len(parts) > 2 {
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

func validateProxy(proxy Proxy, timeout time.Duration) bool {
	proxyURL, err := url.Parse(fmt.Sprintf("%s://%s:%s", proxy.Protocol, proxy.IP, proxy.Port))
	if err != nil {
		debugLog("Invalid proxy URL: %v", err)
		return false
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyURL(proxyURL),
			DisableKeepAlives: true,
		},
		Timeout: timeout,
	}

	req, _ := http.NewRequest("GET", "http://checkip.amazonaws.com", nil)
	req.Header.Set("User-Agent", getRandomUserAgent())

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		debugLog("Proxy %s:%s failed in %v: %v", proxy.IP, proxy.Port, duration, err)
		return false
	}
	defer resp.Body.Close()

	debugLog("Proxy %s:%s succeeded in %v (status: %d)", proxy.IP, proxy.Port, duration, resp.StatusCode)
	return resp.StatusCode == 200
}

func buildChainedProxy(proxies []Proxy, chainSize int) (*http.Client, error) {
	if len(proxies) == 0 {
		return nil, fmt.Errorf("no proxies provided for chain")
	}

	if chainSize <= 0 {
		chainSize = 1
	}
	if chainSize > len(proxies) {
		chainSize = len(proxies)
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(proxies), func(i, j int) {
		proxies[i], proxies[j] = proxies[j], proxies[i]
	})
	selectedProxies := proxies[:chainSize]

	debugLog("Building proxy chain with %d proxies (selected from %d available):", chainSize, len(proxies))
	for i, p := range selectedProxies {
		debugLog("  %d: %s://%s:%s", i+1, p.Protocol, p.IP, p.Port)
	}

	transport := &http.Transport{
		DisableKeepAlives: true,
	}

	if chainSize == 1 {
		proxyURL, err := url.Parse(fmt.Sprintf("%s://%s:%s", selectedProxies[0].Protocol, selectedProxies[0].IP, selectedProxies[0].Port))
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %v", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	} else {
		currentProxy := 0
		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			proxy := selectedProxies[currentProxy]
			currentProxy = (currentProxy + 1) % chainSize
			return url.Parse(fmt.Sprintf("%s://%s:%s", proxy.Protocol, proxy.IP, proxy.Port))
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	debugLog("Proxy chain built successfully")
	return client, nil
}

func scanDomain(client *http.Client, domain string) (string, error) {
	targetURL := "https://" + domain
	debugLog("Initiating scan for: %s", targetURL)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		debugLog("Request creation failed for %s: %v", targetURL, err)
		return "", fmt.Errorf("request creation failed: %v", err)
	}

	ua := getRandomUserAgent()
	req.Header.Set("User-Agent", ua)
	req.Header.Add("Accept-Encoding", "identity")
	debugLog("Using User-Agent: %s", ua)

	startTime := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		debugLog("Request to %s failed after %v: %v", targetURL, duration, err)
		return "", fmt.Errorf("request failed after %v: %v", duration, err)
	}
	defer resp.Body.Close()

	debugLog("Received response from %s - Status: %d, Time: %v", targetURL, resp.StatusCode, duration)

	if debugMode {
		for k, v := range resp.Header {
			debugLog("Response header %s: %v", k, v)
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		debugLog("Failed reading body from %s: %v", targetURL, err)
		return "", fmt.Errorf("reading body failed: %v", err)
	}

	debugLog("Successfully scanned %s (status: %d, length: %d)", targetURL, resp.StatusCode, len(body))
	return fmt.Sprintf("Domain: %s | Status: %d | Length: %d | Sample: %.50s...",
		domain, resp.StatusCode, len(body), string(body)), nil
}

func scanDomainWithRetry(client *http.Client, domain string, maxRetries int) (string, error) {
	debugLog("Starting scan for %s (max retries: %d)", domain, maxRetries)

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		debugLog("Attempt %d/%d for %s", i+1, maxRetries, domain)
		result, err := scanDomain(client, domain)
		if err == nil {
			return result, nil
		}
		lastErr = err
		retryDelay := time.Second * time.Duration(i+1)
		debugLog("Attempt %d failed: %v. Retrying in %v", i+1, err, retryDelay)
		time.Sleep(retryDelay)
	}

	return "", fmt.Errorf("after %d attempts: %v", maxRetries, lastErr)
}

func updateProxies(c *cli.Context) error {
	url := "https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt"
	debugLog("Fetching proxies from: %s", url)

	proxies, err := fetchProxiesFromURL(url)
	if err != nil {
		return fmt.Errorf("failed to fetch proxies: %v", err)
	}

	err = saveProxiesToFile(proxies, "proxies.txt")
	if err != nil {
		return fmt.Errorf("failed to save proxies: %v", err)
	}

	fmt.Printf("Downloaded and saved %d HTTP proxies\n", len(proxies))
	return nil
}

func validateProxies(c *cli.Context) error {
	proxies, err := loadProxiesFromFile("proxies.txt")
	if err != nil {
		return fmt.Errorf("failed to load proxies: %v", err)
	}

	total := len(proxies)
	fmt.Printf("Total proxies to validate: %d\n", total)

	proxyChan := make(chan Proxy, 1000)
	validChan := make(chan Proxy, 1000)
	progressChan := make(chan int, 1000)

	numWorkers := 50
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for proxy := range proxyChan {
				if validateProxy(proxy, 10*time.Second) {
					validChan <- proxy
				}
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

	validProxies := []Proxy{}
	go func() {
		for proxy := range validChan {
			validProxies = append(validProxies, proxy)
		}
	}()

	count := 0
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for count < total {
		select {
		case <-ticker.C:
			percentage := float64(count) * 100 / float64(total)
			fmt.Printf("\rProgress: [%d/%d] %.2f%%", count, total, percentage)
		case <-progressChan:
			count++
		}
	}

	wg.Wait()
	close(validChan)
	close(progressChan)

	fmt.Printf("\rProgress: [%d/%d] 100.00%%\n", total, total)
	fmt.Printf("Validation completed. Found %d valid proxies.\n", len(validProxies))

	err = saveProxiesToFile(validProxies, "valid_proxies.txt")
	if err != nil {
		return fmt.Errorf("failed to save valid proxies: %v", err)
	}

	return nil
}

func scanDomains(domainsFile string, chainSize int) error {
	debugLog("Reading domains from file: %s", domainsFile)
	domainsBytes, err := os.ReadFile(domainsFile)
	if err != nil {
		return fmt.Errorf("failed to read domains file: %v", err)
	}

	domains := strings.Split(string(domainsBytes), "\n")
	debugLog("Loaded %d domains to scan", len(domains))

	validProxies, err := loadProxiesFromFile("valid_proxies.txt")
	if err != nil {
		return fmt.Errorf("failed to load valid proxies: %v", err)
	}

	if len(validProxies) == 0 {
		return fmt.Errorf("no valid proxies available")
	}

	debugLog("Building proxy client with chain size %d (from %d available proxies)", chainSize, len(validProxies))
	client, err := buildChainedProxy(validProxies, chainSize)
	if err != nil {
		return fmt.Errorf("failed to build proxy client: %v", err)
	}

	debugLog("Testing proxy connectivity with https://httpbin.org/ip...")
	testResult, testErr := scanDomainWithRetry(client, "httpbin.org/ip", 2)
	if testErr != nil {
		debugLog("Proxy test failed: %v", testErr)
		return fmt.Errorf("proxy test failed: %v", testErr)
	}
	debugLog("Proxy test successful: %s", testResult)

	for _, domain := range domains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}

		debugLog("\nStarting scan for domain: %s", domain)
		result, err := scanDomainWithRetry(client, domain, 2)
		if err != nil {
			debugLog("Scan failed for %s: %v", domain, err)
			fmt.Printf("Error scanning %s: %v\n", domain, err)
			continue
		}

		fmt.Println(result)
	}

	return nil
}

func main() {
	app := &cli.App{
		Name:    "proxyctl",
		Version: "1.0",
		Usage:   "A tool for managing and using proxy servers",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "Enable debug mode",
			},
		},
		Before: func(c *cli.Context) error {
			debugMode = c.Bool("debug")
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:   "update",
				Usage:  "Fetch and save the latest list of HTTP proxies",
				Action: updateProxies,
			},
			{
				Name:   "validate",
				Usage:  "Validate all saved proxies and filter only working ones",
				Action: validateProxies,
			},
			{
				Name:  "scan",
				Usage: "Scan domains via proxy chains with random User-Agent",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "domains",
						Usage:    "Path to file containing domains to scan",
						Required: true,
					},
					&cli.IntFlag{
						Name:    "chain",
						Aliases: []string{"c"},
						Usage:   "Number of proxies in chain (1-10)",
						Value:   3,
					},
				},
				Action: func(c *cli.Context) error {
					chainSize := c.Int("chain")
					if chainSize < 1 || chainSize > 10 {
						return cli.Exit("Chain size must be between 1 and 10", 1)
					}

					domainsFile := c.String("domains")

					return scanDomains(domainsFile, chainSize)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
