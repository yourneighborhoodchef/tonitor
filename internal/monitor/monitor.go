package monitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yourneighborhoodchef/tonitor/internal/client"
	"github.com/yourneighborhoodchef/tonitor/internal/headers"
	"github.com/yourneighborhoodchef/tonitor/internal/ratelimit"
)

func MonitorProduct(tcin string, targetDelayMs time.Duration, initialConcurrency int) {
	var prevStatus string
	resultChan := make(chan Result)

	var statusMutex sync.Mutex
	var totalRequests int64
	targetRPS := 1000.0 / float64(targetDelayMs.Milliseconds())

	jar := ratelimit.NewTokenJar(targetRPS, initialConcurrency*2)
	defer jar.Stop()

	var requestsInWindow int64
	var requestWindowMutex sync.Mutex
	requestWindowStart := time.Now()

	workerControl := make(chan int)
	var activeWorkers int32 = 0

	go func() {
		numWorkers := initialConcurrency
		atomic.StoreInt32(&activeWorkers, int32(numWorkers))

		for i := 0; i < numWorkers; i++ {
			workerControl <- i
		}

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			requestWindowMutex.Lock()
			windowDuration := time.Since(requestWindowStart).Seconds()
			reqs := atomic.LoadInt64(&requestsInWindow)
			currentRPS := float64(reqs) / windowDuration

			atomic.StoreInt64(&requestsInWindow, 0)
			requestWindowStart = time.Now()
			requestWindowMutex.Unlock()

			if reqs == 0 {
				continue
			}

			currentWorkers := atomic.LoadInt32(&activeWorkers)

			var desiredWorkers int32

			if currentRPS < targetRPS*0.8 {
				desiredWorkers = int32(float64(currentWorkers) * 1.5)
			} else if currentRPS > targetRPS*1.2 {
				desiredWorkers = int32(float64(currentWorkers) * 0.8)
			} else {
				desiredWorkers = currentWorkers
			}

			if desiredWorkers < 1 {
				desiredWorkers = 1
			} else if desiredWorkers > 50 {
				desiredWorkers = 50
			}

			workerDiff := desiredWorkers - currentWorkers
			if workerDiff > 3 {
				workerDiff = 3
			} else if workerDiff < -3 {
				workerDiff = -3
			}

			newWorkerCount := currentWorkers + workerDiff

			if newWorkerCount > currentWorkers {
				for i := 0; i < int(newWorkerCount-currentWorkers); i++ {
					workerID := int(currentWorkers) + i
					workerControl <- workerID
					atomic.AddInt32(&activeWorkers, 1)
				}
			} else if newWorkerCount < currentWorkers {
				atomic.StoreInt32(&activeWorkers, newWorkerCount)
			}
		}
	}()

	workerFactory := func(workerID int) {
		httpClient, err := client.CreateClient()
		if err != nil {
			return
		}

		workerRequests := 0

		var totalDuration time.Duration
		var headerGenDuration time.Duration

		for {
			if int32(workerID) >= atomic.LoadInt32(&activeWorkers) {
				return
			}

			jar.WaitForToken()

			headerStart := time.Now()
			_ = headers.BuildHeaders(tcin)
			headerTime := time.Since(headerStart)
			headerGenDuration += headerTime

			requestStart := time.Now()
			ok, err := client.CheckStock(httpClient, tcin)
			requestDuration := time.Since(requestStart)
			totalDuration += requestDuration

			status := "out-of-stock"
			if err != nil {
				if errors.Is(err, client.ErrProxyBlocked) {
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

			resultChan <- Result{
				Status:    status,
				ProductID: tcin,
				Timestamp: time.Now().Unix(),
				WorkerID:  workerID,
				Latency:   requestDuration.Seconds(),
			}

			if errors.Is(err, client.ErrProxyBlocked) {
				newClient, cerr := client.CreateClient()
				if cerr != nil {
					return
				}
				httpClient = newClient
			}

			time.Sleep(time.Duration(10+rand.Intn(40)) * time.Millisecond)
		}
	}

	go func() {
		for workerID := range workerControl {
			go workerFactory(workerID)
		}
	}()

	heartbeatInterval := targetDelayMs
	lastPublishTime := time.Now().Add(-heartbeatInterval)

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
			resultObj := map[string]interface{}{
				"status":     r.Status,
				"product_id": r.ProductID,
				"timestamp":  r.Timestamp,
				"retailer":   "target",
				"last_check": float64(now.UnixNano()) / 1e9,
				"in_stock":   r.Status == "in-stock",
				"worker_id":  r.WorkerID,
				"latency":    r.Latency,
			}

			resultJSON, err := json.Marshal(resultObj)
			if err != nil {
				fmt.Printf("Error serializing message: %v\n", err)
			} else {
				fmt.Println(string(resultJSON))
			}

			lastPublishTime = now
			prevStatus = r.Status
		}
		statusMutex.Unlock()
	}
}