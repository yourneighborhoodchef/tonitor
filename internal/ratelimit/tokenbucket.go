package ratelimit

import (
	"sync"
	"time"
)

type TokenJar struct {
	refillIntervalMs int64
	tokensPerRefill  int
	maxTokens        int
	tokens           int
	mu               sync.Mutex
	tokensAvailable  chan struct{}
	done             chan struct{}
}

func NewTokenJar(targetRPS float64, burstLimit int) *TokenJar {
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

	jar := &TokenJar{
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

func (tj *TokenJar) refiller() {
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

			if prevTokens == 0 && tj.tokens > 0 {
				select {
				case tj.tokensAvailable <- struct{}{}:
				default:
				}
			}
			tj.mu.Unlock()

		case <-tj.done:
			return
		}
	}
}

func (tj *TokenJar) getToken() bool {
	tj.mu.Lock()
	if tj.tokens > 0 {
		tj.tokens--
		tj.mu.Unlock()
		return true
	}
	tj.mu.Unlock()
	return false
}

func (tj *TokenJar) GetStats() (tokens, maxTokens, tokensPerRefill int, refillIntervalMs int64) {
	tj.mu.Lock()
	defer tj.mu.Unlock()
	return tj.tokens, tj.maxTokens, tj.tokensPerRefill, tj.refillIntervalMs
}

func (tj *TokenJar) WaitForToken() {
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

func (tj *TokenJar) Stop() {
	close(tj.done)
}