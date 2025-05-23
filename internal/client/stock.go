package client

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/yourneighborhoodchef/tonitor/internal/headers"
	"github.com/yourneighborhoodchef/tonitor/internal/logging"
)

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

var (
	prevStatusCode  int
	statusCodeMutex sync.Mutex
)

func extractStockStatus(body []byte) (bool, error) {
	var s stockStatus
	if err := json.Unmarshal(body, &s); err != nil {
		sample := string(body)
		if len(sample) > 200 {
			sample = sample[:200] + "..."
		}
		logging.Printf("JSON parse error: %v\nSample response: %s", err, sample)
		return false, err
	}

	st := s.Data.Product.Fulfillment.ShippingOptions.AvailabilityStatus
	switch st {
	case "IN_STOCK", "LIMITED_STOCK", "PRE_ORDER_SELLABLE":
		return true, nil
	}
	return false, nil
}

func CheckStock(client *ProxiedClient, tcin string) (bool, error) {
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

	req.Header = headers.BuildHeaders(tcin)

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
			headers.ResetProfilePool()
			go headers.InitProfilePool(50)
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
		req2.Header = headers.BuildHeaders(proxyTCIN)
		resp2, err := client.Do(req2)
		if err != nil {
			return false, err
		}
		defer resp2.Body.Close()

		remaining := RemoveProxy(client.ProxyURL)
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
				headers.ResetProfilePool()
				go headers.InitProfilePool(50)
			}

			prevStatusCode = 404
		}
	}
	return extractStockStatus(body)
}