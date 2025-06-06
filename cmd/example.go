package main

import (
	"fmt"
	"time"

	"github.com/yourneighborhoodchef/tonitor/internal/client"
	"github.com/yourneighborhoodchef/tonitor/internal/headers"
	"github.com/yourneighborhoodchef/tonitor/internal/monitor"
)

func main() {
	fmt.Println("Testing tonitor with TCIN: 50516598")

	// Configure proxy list (optional)
	// Uncomment and add your proxies in the format: "protocol://username:password@host:port"
	// or "protocol://host:port" for proxies without authentication
	proxies := []string{
		// "http://proxy1.example.com:8080",
		// "http://user:pass@proxy2.example.com:3128",
		// "socks5://proxy3.example.com:1080",
	}

	// Set up proxy rotation if proxies are provided
	if len(proxies) > 0 {
		client.SetProxyList(proxies)
		fmt.Printf("Configured %d proxies for rotation\n", len(proxies))
	} else {
		fmt.Println("Running without proxies (direct connection)")
	}

	// Initialize header profiles
	headers.InitProfilePool(15000)

	// Set up monitoring parameters
	tcin := "50516598"
	delayMs := 3500 // 3500ms between checks
	workers := 3    // Start with 3 workers

	fmt.Printf("Monitoring TCIN %s with %d second intervals using %d workers\n",
		tcin, delayMs/1000, workers)

	// Example: Use custom compression types
	// Uncomment one of the following lines to test different compression types:

	// Only zstd compression:
	//monitor.MonitorProduct(tcin, time.Duration(delayMs)*time.Millisecond, workers, "zstd")

	// Multiple compression types:
	//monitor.MonitorProduct(tcin, time.Duration(delayMs)*time.Millisecond, workers, "gzip, deflate, br, zstd")

	// Brotli only:
	// monitor.MonitorProduct(tcin, time.Duration(delayMs)*time.Millisecond, workers, "br")

	// Start monitoring with default compression types (random selection)
	monitor.MonitorProduct(tcin, time.Duration(delayMs)*time.Millisecond, workers)
}
