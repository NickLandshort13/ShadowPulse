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
	"golang.org/x/net/proxy"
)

var proxyChan = make(chan Proxy, 100)

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
		if len(parts) == 2 {
			proxies = append(proxies, Proxy{IP: parts[0], Port: parts[1], Protocol: "http"})
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
		_, err := file.WriteString(fmt.Sprintf("%s:%s\n", p.IP, p.Port))
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
		if len(parts) == 2 {
			proxies = append(proxies, Proxy{IP: parts[0], Port: parts[1], Protocol: "http"})
		}
	}

	return proxies, scanner.Err()
}

func validateProxy(proxy Proxy, timeout time.Duration) bool {
	proxyURL, _ := url.Parse(fmt.Sprintf("http://%s:%s", proxy.IP, proxy.Port))
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: nil,
		},
		Timeout: timeout,
	}

	req, _ := http.NewRequest("GET", "http://checkip.amazonaws.com", nil)
	req.Header.Set("User-Agent", getRandomUserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func buildChainedProxy(proxies []Proxy) (*http.Client, error) {
	if len(proxies) == 0 {
		return nil, fmt.Errorf("no proxies provided for chain")
	}

	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%s", proxies[0].IP, proxies[0].Port), nil, proxy.Direct)
	if err != nil {
		return nil, err
	}

	for i := 1; i < len(proxies); i++ {
		nextDialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%s", proxies[i].IP, proxies[i].Port), nil, proxy.Direct)
		if err != nil {
			continue
		}
		dialer = nextDialer
	}

	transport := &http.Transport{
		Dial: dialer.Dial,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}, nil
}

func scanDomain(client *http.Client, domain string) (string, error) {
	req, err := http.NewRequest("GET", "https://"+domain, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", getRandomUserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Domain: %s | Status: %d | Length: %d | Sample: %.50s...", domain, resp.StatusCode, len(body), string(body)), nil
}

func updateProxies(c *cli.Context) error {
	url := "https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt"
	proxies, err := fetchProxiesFromURL(url)
	if err != nil {
		log.Fatalf("Failed to fetch proxies: %v", err)
	}

	err = saveProxiesToFile(proxies, "proxies.txt")
	if err != nil {
		log.Fatalf("Failed to save proxies: %v", err)
	}

	fmt.Printf("Downloaded and saved %d proxies\n", len(proxies))
	return nil
}

func validateProxies(c *cli.Context) error {
	proxies, err := loadProxiesFromFile("proxies.txt")
	if err != nil {
		log.Fatalf("Failed to load proxies: %v", err)
	}

	total := len(proxies)
	fmt.Printf("Total proxies to validate: %d\n", total)

	proxyChan := make(chan Proxy, 1000)
	validChan := make(chan Proxy, 1000)
	progressChan := make(chan int, 1000)

	numWorkers := 1000
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for proxy := range proxyChan {
				if validateProxy(proxy, 5*time.Second) {
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
	ticker := time.NewTicker(100 * time.Millisecond)
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
		log.Fatalf("Failed to save valid proxies: %v", err)
	}

	return nil
}

func scanDomains(c *cli.Context) error {
	domainsFile := c.String("domains")
	if domainsFile == "" {
		log.Fatal("Please provide a domains file using --domains")
	}

	domainsBytes, err := os.ReadFile(domainsFile)
	if err != nil {
		log.Fatalf("Failed to read domains file: %v", err)
	}

	domains := strings.Split(string(domainsBytes), "\n")

	validProxies, err := loadProxiesFromFile("valid_proxies.txt")
	if err != nil {
		log.Fatalf("Failed to load valid proxies: %v", err)
	}

	if len(validProxies) < 2 {
		log.Fatal("Need at least 2 valid proxies to build a chain")
	}

	client, err := buildChainedProxy(validProxies[:2])
	if err != nil {
		log.Fatalf("Failed to build proxy chain: %v", err)
	}

	for _, domain := range domains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}

		result, err := scanDomain(client, domain)
		if err != nil {
			fmt.Printf("Error scanning %s: %v\n", domain, err)
			continue
		}

		fmt.Println(result)
	}

	return nil
}

func main() {
	app := &cli.App{
		Name:  "proxyctl",
		Usage: "A tool for managing and using proxy servers",
		Commands: []*cli.Command{
			{
				Name:   "update",
				Usage:  "Fetch and save the latest list of HTTP proxies from TheSpeedX",
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
				},
				Action: scanDomains,
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
