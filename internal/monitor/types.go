package monitor

type Result struct {
	Status    string
	ProductID string
	Timestamp int64
	WorkerID  int
	Latency   float64
}

type StockMessage struct {
	Status    string  `json:"status"`
	ProductID string  `json:"product_id"`
	Retailer  string  `json:"retailer"`
	LastCheck float64 `json:"last_check"`
	InStock   bool    `json:"in_stock"`
}