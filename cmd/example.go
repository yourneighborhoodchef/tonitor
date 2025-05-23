package main

import (
	"fmt"
	"time"

	"github.com/yourneighborhoodchef/tonitor/internal/headers"
	"github.com/yourneighborhoodchef/tonitor/internal/monitor"
)

func main() {
	fmt.Println("Testing tonitor with TCIN: 50516598")
	
	// Initialize header profiles
	headers.InitProfilePool(15000)
	
	// Set up monitoring parameters
	tcin := "50516598"
	delayMs := 3500 // 3500ms between checks
	workers := 3     // Start with 3 workers
	
	fmt.Printf("Monitoring TCIN %s with %d second intervals using %d workers\n", 
		tcin, delayMs/1000, workers)
	
	// Start monitoring
	monitor.MonitorProduct(tcin, time.Duration(delayMs)*time.Millisecond, workers)
}