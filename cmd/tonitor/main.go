package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/yourneighborhoodchef/tonitor/internal/client"
	"github.com/yourneighborhoodchef/tonitor/internal/headers"
	"github.com/yourneighborhoodchef/tonitor/internal/monitor"
)

func main() {
	rand.Seed(time.Now().UnixNano())

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

	targetDelayMs := 30000
	if len(os.Args) > 2 {
		if d, err := strconv.Atoi(os.Args[2]); err == nil && d > 0 {
			targetDelayMs = d
		}
	}

	if len(os.Args) > 3 {
		var proxyList []string
		for _, p := range strings.Split(os.Args[3], ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				proxyList = append(proxyList, p)
			}
		}
		if len(proxyList) > 0 {
			fmt.Printf("Using %d proxy(ies)\n", len(proxyList))
			client.SetProxyList(proxyList)
		}
	}

	initialWorkers := 5

	headers.InitProfilePool(15000)

	monitor.MonitorProduct(tcin, time.Duration(targetDelayMs)*time.Millisecond, initialWorkers)
}