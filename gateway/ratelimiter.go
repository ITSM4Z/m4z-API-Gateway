package main

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type ClientBucket struct {
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

type RateLimiter struct {
	mu         sync.Mutex
	clients    map[string]*ClientBucket
	maxTokens  float64
	refillRate float64
}

// handles the rate limiting by managing client's bucket tokens
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get the host and discard the port
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		clientIP := host

		rl.mu.Lock()

		now := time.Now()
		bucket, ok := rl.clients[clientIP]

		// if the client is a john doe; create a bucket with its IP
		if !ok {
			bucket = &ClientBucket{
				tokens:     rl.maxTokens,
				lastRefill: now,
			}

			rl.clients[clientIP] = bucket
		}

		bucket.lastSeen = now

		// seconds since last token refill
		elapsed := now.Sub(bucket.lastRefill).Seconds()

		// refill client's tokens every second with the RateLimiter refill rate
		bucket.tokens += elapsed * rl.refillRate

		if bucket.tokens > rl.maxTokens {
			bucket.tokens = rl.maxTokens
		}

		bucket.lastRefill = now

		// if the client consumed all of its tokens, unlock then return a 429 status
		if bucket.tokens < 1 {
			rl.mu.Unlock()
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		// otherwise consume 1 token, unlock then move to the Gateway
		bucket.tokens--
		rl.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) cleanupLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			rl.mu.Lock()
			for clientIP, bucket := range rl.clients {
				if now.Sub(bucket.lastSeen) > 10*time.Minute {
					delete(rl.clients, clientIP)
				}
			}
			rl.mu.Unlock()

		case <-stop:
			return
		}
	}
}
