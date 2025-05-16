package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

var ErrProxyBlocked = errors.New("proxy blocked")
var proxyList []string
var proxyListMutex sync.Mutex
var proxyCounter uint32

type stockStatus struct {
	Data struct {
		Product struct {
			Fulfillment struct {
				ShippingOptions struct {
					AvailabilityStatus string `json:"availability_status"`
				} `json:"shipping_options"`
				SoldOut bool `json:"sold_out"`
			} `json:"fulfillment"`
		} `json:"product"`
	} `json:"data"`
}

func extractStockStatus(body []byte) (bool, error) {
	var s stockStatus
	if err := json.Unmarshal(body, &s); err != nil {
		sample := string(body)
		if len(sample) > 200 {
			sample = sample[:200] + "..."
		}
		fmt.Printf("JSON parse error: %v\nSample response: %s\n", err, sample)
		return false, err
	}

	st := s.Data.Product.Fulfillment.ShippingOptions.AvailabilityStatus
	switch st {
	case "IN_STOCK", "LIMITED_STOCK", "PRE_ORDER_SELLABLE":
		return true, nil
	}
	return false, nil
}

func randomBuildToken(n int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

type viewport struct {
	Width      int
	Height     int
	PixelRatio float64
}

func generateRandomViewport() viewport {
	w := rand.Intn(80) + 360
	h := rand.Intn(276) + 640
	dprChoices := []float64{2, 2.5, 3, 3.5}
	return viewport{Width: w, Height: h, PixelRatio: dprChoices[rand.Intn(len(dprChoices))]}
}

type clientHints struct {
	ConnectionType string
	EffectiveType  string
	Downlink       float64
	RTT            int
	SaveData       string
}

func generateClientHints() clientHints {
	connOpts := []string{"keep-alive", "close", ""}
	effOpts := []string{"slow-2g", "2g", "3g", "4g", ""}
	saveOpts := []string{"on", ""}
	return clientHints{
		ConnectionType: connOpts[rand.Intn(len(connOpts))],
		EffectiveType:  effOpts[rand.Intn(len(effOpts))],
		Downlink:       rand.Float64()*9.9 + 0.1,
		RTT:            rand.Intn(251) + 50,
		SaveData:       saveOpts[rand.Intn(len(saveOpts))],
	}
}

func generateRandomUA() string {
	androidVer := rand.Intn(10) + 8
	architectures := []string{"arm64-v8a", "armeabi-v7a", "x86", "x86_64"}
	arch := architectures[rand.Intn(len(architectures))]

	switch rand.Intn(5) {
	case 0: // Android Chrome
		maj := rand.Intn(11) + 130
		min := rand.Intn(10)
		return fmt.Sprintf(
			"Mozilla/5.0 (Linux; Android %d; AndroidDeviceModel%02d; %s Build/%s) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.%d.0 Mobile Safari/537.36 XTR/%s",
			androidVer, rand.Intn(80)+1, arch, randomBuildToken(5),
			maj, min, randomBuildToken(5),
		)
	case 1: // iOS Safari
		maj := rand.Intn(8) + 12
		min := rand.Intn(10)
		device := fmt.Sprintf("iPhone%02d", rand.Intn(80)+6)
		return fmt.Sprintf(
			"Mozilla/5.0 (%s; CPU %s OS %d_%d like Mac OS X) "+
				"AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%d.%d Mobile/%s Safari/604.1 XTR/%s",
			device, device, maj, min, maj, rand.Intn(2), randomBuildToken(5), randomBuildToken(5),
		)
	case 2: // Samsung Internet
		sbMaj := rand.Intn(7) + 20
		sbMin := rand.Intn(10)
		return fmt.Sprintf(
			"Mozilla/5.0 (Linux; Android %d; SAMSUNG SM-G%03d; %s Build/%s) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/%d.%d "+
				"Chrome/%d.0.%d.0 Mobile Safari/537.36 XTR/%s",
			androidVer, rand.Intn(80)+900, arch, randomBuildToken(5),
			sbMaj, sbMin, rand.Intn(11)+130, rand.Intn(10), randomBuildToken(5),
		)
	case 3: // Android Firefox
		rvMaj := rand.Intn(16) + 115
		rvMin := rand.Intn(10)
		return fmt.Sprintf(
			"Mozilla/5.0 (Android %d; %s; Mobile; rv:%d.%d) Gecko/%d.%d Firefox/%d.%d Build/%s XTR/%s",
			androidVer, arch, rvMaj, rvMin, rvMaj, rvMin, rvMaj, rvMin, randomBuildToken(5), randomBuildToken(5),
		)
	default: // Edge Mobile
		edgeMaj := rand.Intn(11) + 120
		edgeMin := rand.Intn(10)
		return fmt.Sprintf(
			"Mozilla/5.0 (Linux; Android %d; EdgeDeviceModel%02d; %s Build/%s) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.%d.0 Mobile Safari/537.36 EdgA/%d.%d XTR/%s",
			androidVer, rand.Intn(80)+1, arch, randomBuildToken(5),
			rand.Intn(11)+130, rand.Intn(10), edgeMaj, edgeMin, randomBuildToken(5),
		)
	}
}

func generateSecCHUA(ua string) string {
	const fallback = "136.0.0.0"
	ver := fallback
	if idx := strings.Index(ua, "Chrome/"); idx != -1 {
		rest := ua[idx+7:]
		if j := strings.Index(rest, " "); j != -1 {
			ver = rest[:j]
		} else {
			ver = rest
		}
	}
	return fmt.Sprintf(
		`"Not:A-Brand";v="24", "Chromium";v="%s", "Google Chrome";v="%s"`,
		ver, ver,
	)
}

var (
	acceptOpts = []string{
		"application/json",
		"application/json, text/plain, */*",
		"application/json, text/javascript, */*; q=0.01",
	}
	encOpts = []string{
		"gzip, deflate, br",
		"gzip, deflate, br, zstd",
		"br, gzip, deflate",
	}
	langOpts = []string{
		"en-US,en;q=0.9",
		"en-US,en;q=0.8",
		"en-GB,en;q=0.9,en-US;q=0.8",
		"en-CA,en;q=0.9,en-US;q=0.8",
		"en,en-US;q=0.9",
		"en-US,en;q=0.9,es;q=0.8",
		"en-US",
	}
	cacheOpts = []string{
		"max-age=0",
		"no-cache",
		"max-age=0, private, must-revalidate",
		"",
	}
	memOpts = []string{"2", "4", "8"}

	headerOrder = []string{
		"Accept",
		"Accept-Language",
		"Accept-Encoding",
		"User-Agent",
		"Sec-CH-UA",
		"Sec-CH-UA-Mobile",
		"Sec-CH-UA-Platform",
		"Sec-Fetch-Site",
		"Sec-Fetch-Mode",
		"Sec-Fetch-Dest",
		"Sec-CH-Viewport-Width",
		"Sec-CH-Viewport-Height",
		"Sec-CH-DPR",
		"Sec-CH-UA-Netinfo",
		"Sec-CH-UA-Downlink",
		"Sec-CH-UA-RTT",
		"Save-Data",
		"Device-Memory",
		"Connection",
		"Cache-Control",
		"Origin",
		"Referer",
		"Priority",
	}
)

type headerProfile struct {
	ua           string
	secCHUA      string
	viewport     viewport
	hints        clientHints
	acceptIdx    int
	langIdx      int
	encIdx       int
	cacheIdx     int
	memIdx       int
	viewportProb []float64
	hintsProb    []float64
	memProb      float64
}

func generateHeaderProfile() headerProfile {
	ua := generateRandomUA()

	viewportProbs := make([]float64, 3)
	for i := range viewportProbs {
		viewportProbs[i] = rand.Float64()
	}

	hintsProbs := make([]float64, 5)
	for i := range hintsProbs {
		hintsProbs[i] = rand.Float64()
	}

	return headerProfile{
		ua:           ua,
		secCHUA:      generateSecCHUA(ua),
		viewport:     generateRandomViewport(),
		hints:        generateClientHints(),
		acceptIdx:    rand.Intn(len(acceptOpts)),
		langIdx:      rand.Intn(len(langOpts)),
		encIdx:       rand.Intn(len(encOpts)),
		cacheIdx:     rand.Intn(len(cacheOpts)),
		memIdx:       rand.Intn(len(memOpts)),
		viewportProb: viewportProbs,
		hintsProb:    hintsProbs,
		memProb:      rand.Float64(),
	}
}

var headerProfilePool = sync.Pool{
	New: func() interface{} {
		return generateHeaderProfile()
	},
}

func buildHeaders(tcin string) http.Header {
	profile := headerProfilePool.Get().(headerProfile)
	defer headerProfilePool.Put(profile)

	h := http.Header{}
	h.Set("Accept", acceptOpts[profile.acceptIdx])
	h.Set("Accept-Language", langOpts[profile.langIdx])
	h.Set("Accept-Encoding", encOpts[profile.encIdx])
	h.Set("User-Agent", profile.ua)
	h.Set("Sec-CH-UA", profile.secCHUA)
	h.Set("Sec-CH-UA-Mobile", func() string {
		if strings.Contains(profile.ua, "Mobile") {
			return "?1"
		}
		return "?0"
	}())
	h.Set("Sec-CH-UA-Platform", `"`+func() string {
		if strings.Contains(profile.ua, "Android") {
			return "Android"
		}
		return "macOS"
	}()+`"`)
	h.Set("Origin", "https://www.target.com")
	h.Set("Referer", fmt.Sprintf(
		"https://www.target.com/p/2023-panini-select-baseball-trading-card-blaster-box/-/A-%s",
		tcin,
	))
	h.Set("Sec-Fetch-Site", "same-site")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Dest", "empty")
	h.Set("Priority", "u=1,i")

	// Optional Cache-Control
	if cc := cacheOpts[profile.cacheIdx]; cc != "" {
		h.Set("Cache-Control", cc)
	}

	// Viewport hints - use pre-generated probabilities
	if profile.viewportProb[0] < 0.7 {
		h.Set("Sec-CH-Viewport-Width", strconv.Itoa(profile.viewport.Width))
	}
	if profile.viewportProb[1] < 0.7 {
		h.Set("Sec-CH-Viewport-Height", strconv.Itoa(profile.viewport.Height))
	}
	if profile.viewportProb[2] < 0.7 {
		h.Set("Sec-CH-DPR", fmt.Sprintf("%.1f", profile.viewport.PixelRatio))
	}

	// Network/client hints - use pre-generated probabilities
	if profile.hints.ConnectionType != "" && profile.hintsProb[0] < 0.6 {
		h.Set("Connection", profile.hints.ConnectionType)
	}
	if profile.hints.EffectiveType != "" && profile.hintsProb[1] < 0.5 {
		h.Set("Sec-CH-UA-Netinfo", profile.hints.EffectiveType)
	}
	if profile.hintsProb[2] < 0.4 {
		h.Set("Sec-CH-UA-Downlink", fmt.Sprintf("%.1f", profile.hints.Downlink))
	}
	if profile.hintsProb[3] < 0.3 {
		h.Set("Sec-CH-UA-RTT", strconv.Itoa(profile.hints.RTT))
	}
	if profile.hints.SaveData == "on" {
		h.Set("Save-Data", "on")
	}

	// Device memory hint
	if profile.memProb < 0.3 {
		h.Set("Device-Memory", memOpts[profile.memIdx])
	}

	// Set header order
	h[http.HeaderOrderKey] = headerOrder

	return h
}

var (
	prevStatusCode  int
	statusCodeMutex sync.Mutex
)

func checkStock(client *ProxiedClient, tcin string) (bool, error) {
	url := fmt.Sprintf(
		"https://redsky.target.com/redsky_aggregations/v1/web/product_fulfillment_v1"+
			"?key=9f36aeafbe60771e321a7cc95a78140772ab3e96"+
			"&is_bot=false&tcin=%s&store_id=1077&zip=27019&state=NC"+
			"&channel=WEB&page=%%2Fp%%2FA-%s", tcin, tcin,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	req.Header = buildHeaders(tcin)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == 404 {
		fmt.Printf("404 Not Found: Product %s may not exist (via proxy %s)\n", tcin, client.ProxyURL)
		statusCodeMutex.Lock()
		if prevStatusCode == 200 {
			fmt.Printf("Got 404 after previous 200 response, regenerating header profiles\n")
			headerProfilePool = sync.Pool{New: func() interface{} { return generateHeaderProfile() }}
			go initHeaderProfilePool(50)
		}
		prevStatusCode = 404
		statusCodeMutex.Unlock()

		proxyTCIN := "10805587"
		fmt.Printf("Checking proxy with test TCIN: %s\n", proxyTCIN)
		url2 := fmt.Sprintf(
			"https://redsky.target.com/redsky_aggregations/v1/web/product_fulfillment_v1"+
				"?key=9f36aeafbe60771e321a7cc95a78140772ab3e96"+
				"&is_bot=false&tcin=%s&store_id=1077&zip=27019&state=NC"+
				"&channel=WEB&page=%%2Fp%%2FA-%s", proxyTCIN, proxyTCIN,
		)
		req2, err := http.NewRequest(http.MethodGet, url2, nil)
		if err != nil {
			return false, err
		}
		req2.Header = buildHeaders(proxyTCIN)
		resp2, err := client.Do(req2)
		if err != nil {
			return false, err
		}
		defer resp2.Body.Close()

		proxyListMutex.Lock()
		if client.ProxyURL != "" {
			for i, p := range proxyList {
				if p == client.ProxyURL {
					proxyList = append(proxyList[:i], proxyList[i+1:]...)
					break
				}
			}
		}
		remaining := len(proxyList)
		proxyListMutex.Unlock()
		if resp2.StatusCode == 404 {
			fmt.Printf("Proxy appears blocked: test TCIN %s also returned 404 (proxy %s)\n", proxyTCIN, client.ProxyURL)
			if remaining == 0 {
				time.Sleep(3500 * time.Millisecond)
			}
			return false, ErrProxyBlocked
		}
		if remaining == 0 {
			time.Sleep(3500 * time.Millisecond)
		}
		return false, nil
	}

	statusCodeMutex.Lock()
	defer statusCodeMutex.Unlock()

	if resp.StatusCode == 200 {
		prevStatusCode = 200
	} else {
		fmt.Printf("Unexpected status code: %d\n", resp.StatusCode)
		sample := string(body)
		if len(sample) > 200 {
			sample = sample[:200] + "..."
		}
		fmt.Printf("Response body: %s\n", sample)

		if resp.StatusCode == 404 {
			fmt.Printf("404 Not Found: Product %s may not exist\n", tcin)

			if prevStatusCode == 200 {
				fmt.Printf("Got 404 after previous 200 response, regenerating header profiles\n")
				headerProfilePool = sync.Pool{
					New: func() interface{} {
						return generateHeaderProfile()
					},
				}
				go initHeaderProfilePool(50)
			}

			prevStatusCode = 404
		}
	}
	return extractStockStatus(body)
}

type result struct {
	Status    string
	ProductID string
	Timestamp int64
}

type StockMessage struct {
	Status    string  `json:"status"`
	ProductID string  `json:"product_id"`
	Retailer  string  `json:"retailer"`
	LastCheck float64 `json:"last_check"`
	InStock   bool    `json:"in_stock"`
}

type ProxiedClient struct {
	tls_client.HttpClient
	ProxyURL string
}

func createClient() (*ProxiedClient, error) {
	jar := tls_client.NewCookieJar()
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(jar),
	}
	var proxyURL string
	proxyListMutex.Lock()
	if len(proxyList) > 0 {
		idx := atomic.AddUint32(&proxyCounter, 1)
		proxyURL = proxyList[int(idx-1)%len(proxyList)]
		options = append(options, tls_client.WithProxyUrl(proxyURL))
	}
	proxyListMutex.Unlock()
	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return nil, err
	}
	return &ProxiedClient{HttpClient: client, ProxyURL: proxyURL}, nil
}

type tokenJar struct {
	refillIntervalMs int64
	tokensPerRefill  int
	maxTokens        int
	tokens           int
	mu               sync.Mutex
	tokensAvailable  chan struct{}
	done             chan struct{}
}

func newTokenJar(targetRPS float64, burstLimit int) *tokenJar {
	refillIntervalMs := int64(1000 / targetRPS)
	if refillIntervalMs < 10 {
		refillIntervalMs = 10
	}

	tokensPerRefill := 1
	if targetRPS > 10 {
		tokensPerRefill = int(targetRPS / 5)
		refillIntervalMs = int64(float64(tokensPerRefill) * 1000 / float64(targetRPS))
	}

	if burstLimit <= 0 {
		burstLimit = int(targetRPS * 2)
	}
	if burstLimit < tokensPerRefill {
		burstLimit = tokensPerRefill
	}

	jar := &tokenJar{
		refillIntervalMs: refillIntervalMs,
		tokensPerRefill:  tokensPerRefill,
		maxTokens:        burstLimit,
		tokens:           burstLimit / 2,
		tokensAvailable:  make(chan struct{}, 1),
		done:             make(chan struct{}),
	}

	go jar.refiller()

	return jar
}

func (tj *tokenJar) refiller() {
	ticker := time.NewTicker(time.Duration(tj.refillIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tj.mu.Lock()
			prevTokens := tj.tokens
			tj.tokens += tj.tokensPerRefill
			if tj.tokens > tj.maxTokens {
				tj.tokens = tj.maxTokens
			}

			// Notify if we went from 0 to having tokens
			if prevTokens == 0 && tj.tokens > 0 {
				select {
				case tj.tokensAvailable <- struct{}{}:
				default:
					// Channel already has notification
				}
			}
			tj.mu.Unlock()

		case <-tj.done:
			return
		}
	}
}

func (tj *tokenJar) getToken() bool {
	tj.mu.Lock()
	if tj.tokens > 0 {
		tj.tokens--
		tj.mu.Unlock()
		return true
	}
	tj.mu.Unlock()
	return false
}

func (tj *tokenJar) getStats() (tokens, maxTokens, tokensPerRefill int, refillIntervalMs int64) {
	tj.mu.Lock()
	defer tj.mu.Unlock()
	return tj.tokens, tj.maxTokens, tj.tokensPerRefill, tj.refillIntervalMs
}

func (tj *tokenJar) waitForToken() {
	if tj.getToken() {
		return
	}

	for {
		select {
		case <-tj.tokensAvailable:
			if tj.getToken() {
				return
			}
		case <-time.After(time.Duration(tj.refillIntervalMs) * time.Millisecond):
			if tj.getToken() {
				return
			}
		}
	}
}

func (tj *tokenJar) stop() {
	close(tj.done)
}

func monitorProduct(tcin string, targetDelayMs time.Duration, initialConcurrency int) {
	var prevStatus string
	resultChan := make(chan result)

	var statusMutex sync.Mutex
	var totalRequests int64
	targetRPS := 1000.0 / float64(targetDelayMs.Milliseconds())

	jar := newTokenJar(targetRPS, initialConcurrency*2)
	defer jar.stop()

	var requestsInWindow int64
	var requestWindowMutex sync.Mutex
	requestWindowStart := time.Now()

	workerControl := make(chan int)
	var activeWorkers int32 = 0

	go func() {
		numWorkers := initialConcurrency
		atomic.StoreInt32(&activeWorkers, int32(numWorkers))

		// Start initial workers
		for i := 0; i < numWorkers; i++ {
			workerControl <- i
		}

		// Continuously adjust worker count based on load
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Calculate current requests per second
			requestWindowMutex.Lock()
			windowDuration := time.Since(requestWindowStart).Seconds()
			reqs := atomic.LoadInt64(&requestsInWindow)
			currentRPS := float64(reqs) / windowDuration

			// Reset the window
			atomic.StoreInt64(&requestsInWindow, 0)
			requestWindowStart = time.Now()
			requestWindowMutex.Unlock()

			if reqs == 0 {
				continue // No data yet or no requests in window
			}

			currentWorkers := atomic.LoadInt32(&activeWorkers)

			// The token jar handles the exact rate limiting, but we adjust workers
			// based on whether we need more parallelism or less

			// Calculate ideal workers: if we're hitting our RPS target,
			// adjust worker count based on whether workers are waiting too long
			// for tokens (need fewer workers) or handling requests efficiently (keep same)

			var desiredWorkers int32

			// If we're significantly under our target RPS, add workers
			if currentRPS < targetRPS*0.8 {
				// Add workers to handle more requests in parallel
				desiredWorkers = int32(float64(currentWorkers) * 1.5)
			} else if currentRPS > targetRPS*1.2 {
				// If we're over the target, reduce workers
				desiredWorkers = int32(float64(currentWorkers) * 0.8)
			} else {
				// We're close to the target, keep the same
				desiredWorkers = currentWorkers
			}

			// Maintain reasonable worker limits
			if desiredWorkers < 1 {
				desiredWorkers = 1
			} else if desiredWorkers > 50 {
				desiredWorkers = 50
			}

			// Apply changes gradually
			workerDiff := desiredWorkers - currentWorkers
			if workerDiff > 3 {
				workerDiff = 3 // Add max 3 workers at once
			} else if workerDiff < -3 {
				workerDiff = -3 // Remove max 3 workers at once
			}

			newWorkerCount := currentWorkers + workerDiff

			// Apply the worker scaling
			if newWorkerCount > currentWorkers {
				// Add workers
				for i := 0; i < int(newWorkerCount-currentWorkers); i++ {
					workerID := int(currentWorkers) + i
					workerControl <- workerID
					atomic.AddInt32(&activeWorkers, 1)
				}
			} else if newWorkerCount < currentWorkers {
				// Signal to reduce workers (will stop naturally)
				atomic.StoreInt32(&activeWorkers, newWorkerCount)
			}
		}
	}()

	// Worker factory function
	workerFactory := func(workerID int) {
		// Each worker gets its own client with separate cookie jar
		client, err := createClient()
		if err != nil {
			return
		}

		workerRequests := 0

		// Track request durations for this worker
		var totalDuration time.Duration
		var headerGenDuration time.Duration

		for {
			// Check if this worker should exit due to scaling down
			if int32(workerID) >= atomic.LoadInt32(&activeWorkers) {
				return
			}

			// Wait for a token from the jar before making a request
			jar.waitForToken()

			// First measure just the header generation time separately
			headerStart := time.Now()
			_ = buildHeaders(tcin) // Just generate but don't use
			headerTime := time.Since(headerStart)
			headerGenDuration += headerTime

			// Now time the whole request
			requestStart := time.Now()
			ok, err := checkStock(client, tcin)
			requestDuration := time.Since(requestStart)
			totalDuration += requestDuration

			status := "out-of-stock"
			if err != nil {
				if errors.Is(err, ErrProxyBlocked) {
					status = "proxy-blocked"
				} else {
					status = "error"
				}
			} else if ok {
				status = "in-stock"
			}

			workerRequests++
			atomic.AddInt64(&totalRequests, 1)
			atomic.AddInt64(&requestsInWindow, 1)

			resultChan <- result{
				Status:    status,
				ProductID: tcin,
				Timestamp: time.Now().Unix(),
			}

			// If the proxy was blocked, create a new client to pick the next proxy
			if errors.Is(err, ErrProxyBlocked) {
				newClient, cerr := createClient()
				if cerr != nil {
					return
				}
				client = newClient
			}

			// Add slight jitter between requests (10-50ms)
			// This prevents all workers from synchronizing
			time.Sleep(time.Duration(10+rand.Intn(40)) * time.Millisecond)
		}
	}

	// Launch workers as signaled
	go func() {
		for workerID := range workerControl {
			go workerFactory(workerID)
		}
	}()

	// Process results
	// Heartbeat interval to ensure periodic updates
	heartbeatInterval := targetDelayMs
	lastPublishTime := time.Now().Add(-heartbeatInterval)

	// Send initial status message to stdout
	initialStatusMsg := map[string]interface{}{
		"status":     "initializing",
		"product_id": tcin,
		"retailer":   "target",
		"timestamp":  time.Now().Unix(),
		"last_check": float64(time.Now().UnixNano()) / 1e9,
		"in_stock":   false,
		"latency":    0.0,
		"worker_id":  0,
	}
	initialJSON, _ := json.Marshal(initialStatusMsg)
	fmt.Println(string(initialJSON))

	for r := range resultChan {
		statusMutex.Lock()
		now := time.Now()
		if r.Status != prevStatus || r.Status == "error" || now.Sub(lastPublishTime) >= heartbeatInterval {
			// Create a result object
			resultObj := map[string]interface{}{
				"status":     r.Status,
				"product_id": r.ProductID,
				"timestamp":  r.Timestamp,
				"retailer":   "target",
				"last_check": float64(now.UnixNano()) / 1e9,
				"in_stock":   r.Status == "in-stock",
				"worker_id":  0,
				"latency":    0.1, // Dummy value
			}

			// Convert to JSON and print to stdout
			resultJSON, err := json.Marshal(resultObj)
			if err != nil {
				fmt.Printf("Error serializing message: %v\n", err)
			} else {
				// Print the JSON to stdout
				fmt.Println(string(resultJSON))
			}

			// Update last publish time and status
			lastPublishTime = now
			prevStatus = r.Status
		}
		statusMutex.Unlock()
	}
}

func initHeaderProfilePool(count int) {
	profiles := make([]interface{}, count)
	for i := 0; i < count; i++ {
		profiles[i] = generateHeaderProfile()
	}
	for _, profile := range profiles {
		headerProfilePool.Put(profile)
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// Check for help flag
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			fmt.Println("Usage: tonitor <TCIN> [delay_ms] [proxy_list]")
			fmt.Println("  TCIN: Target product ID to monitor")
			fmt.Println("  delay_ms: Delay between checks in milliseconds (default: 30000)")
			fmt.Println("  proxy_list: Comma-separated list of proxy URLs")
			return
		}
	}

	if len(os.Args) < 2 {
		fmt.Println("Error: Missing TCIN parameter. Use -h for help.")
		return
	}

	tcin := os.Args[1]

	targetDelayMs := 30000 // Default to 30 seconds between requests (0.033 RPS)
	if len(os.Args) > 2 {
		if d, err := strconv.Atoi(os.Args[2]); err == nil && d > 0 {
			targetDelayMs = d
		}
	}

	if len(os.Args) > 3 {
		for _, p := range strings.Split(os.Args[3], ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				proxyList = append(proxyList, p)
			}
		}
		if len(proxyList) > 0 {
			fmt.Printf("Using %d proxy(ies)\n", len(proxyList))
		}
	}

	initialWorkers := 5

	initHeaderProfilePool(15000)

	monitorProduct(tcin, time.Duration(targetDelayMs)*time.Millisecond, initialWorkers)
}
