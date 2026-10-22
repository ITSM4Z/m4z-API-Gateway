package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ctxKey string

const targetKey ctxKey = "target"

type ClientBucket struct {
	tokens     float64
	lastRefill time.Time
}

type RateLimiter struct {
	mu         sync.Mutex
	clients    map[string]*ClientBucket
	maxTokens  float64
	refillRate float64
	next       http.Handler
}

type GatewayRouter struct {
	mu     sync.RWMutex
	routes map[string]*url.URL
	proxy  *httputil.ReverseProxy
}

// handles the rate limiting by managing client's bucket tokens
// to add: goroutine that checks if the bucket last refill > 10 minutes, delete the bucket to avoid memory overhead.
func (rl *RateLimiter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// get the host and discard the port
	host, _, err := net.SplitHostPort(r.RemoteAddr)

	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		fmt.Printf("An Error occured while getting client's IP: %v", err)
		return
	}

	clientIP := host

	rl.mu.Lock()

	bucket, ok := rl.clients[clientIP]

	// if the client is a john doe; create a bucket with its IP
	if !ok {
		bucket = &ClientBucket{
			tokens:     rl.maxTokens,
			lastRefill: time.Now(),
		}

		rl.clients[clientIP] = bucket
	}

	// seconds since last token refill
	elapsed := time.Since(bucket.lastRefill).Seconds()

	// refill client's tokens every second with the RateLimiter refill rate
	bucket.tokens += elapsed * rl.refillRate

	if bucket.tokens > rl.maxTokens {
		bucket.tokens = rl.maxTokens
	}

	bucket.lastRefill = time.Now()

	// if the client consumed all of its tokens, unlock then return a 429 status
	if bucket.tokens < 1.0 {
		rl.mu.Unlock()
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}

	// otherwise consume 1.0 token, unlock then move to the Gateway
	bucket.tokens -= 1.0
	rl.mu.Unlock()
	rl.next.ServeHTTP(w, r)
}

// checks if the requested path exists in the Gateway's routes
// if so, move to the proxy. Otherwise return 404 status
func (g *GatewayRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.RLock()

	for prefix, targetURL := range g.routes {
		if strings.HasPrefix(r.URL.Path, prefix) {
			ctx := context.WithValue(r.Context(), targetKey, targetURL)
			g.mu.RUnlock()
			g.proxy.ServeHTTP(w, r.WithContext(ctx))
			return
		}
	}
	g.mu.RUnlock()
	http.Error(w, "Route Not Found", http.StatusNotFound)
}

func main() {
	rawRoutes := map[string]string{
		"/users":  "http://localhost:9001",
		"/orders": "http://localhost:9002",
	}

	serverRoutes := make(map[string]*url.URL)

	for prefix, targetStr := range rawRoutes {
		serverURL, err := url.Parse(targetStr)

		if err != nil {
			fmt.Printf("An Error occured while parsing (%s): %v", targetStr, err)
			return
		}

		serverRoutes[prefix] = serverURL
	}

	// The pool of reusable TCP Pipes the Gateway will be using when talking to :9000
	gatewayTransport := http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second, // Time to wait if the Backend is Idle
		ResponseHeaderTimeout: 2 * time.Second,  // Time to wait for the server's response headers after fully writing the request (Including body)
		ExpectContinueTimeout: 2 * time.Second,  // Time to wait for the first server's response headers after fully writing the request headers if the request have "Expect: 100 Continue"
	}

	gatewayReverseProxy := httputil.ReverseProxy{
		Transport: &gatewayTransport,
		Rewrite: func(r *httputil.ProxyRequest) {
			targetURL, ok := r.In.Context().Value(targetKey).(*url.URL)
			if !ok {
				return
			}

			r.SetXForwarded()
			r.SetURL(targetURL)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// The code below is in case we wanted to use the http.ResponseWriter instead of http.Error
			// h := w.Header()
			// h.Del("Content-Length")
			// h.Set("Content-Type", "text/plain; charset=utf-8")
			// h.Set("X-Content-Type-Options", "nosniff")
			// w.WriteHeader(http.StatusBadGateway)

			http.Error(w, "Bad Gateway", http.StatusBadGateway)
			fmt.Println(err.Error())
		},
	}

	gatewayRouter := GatewayRouter{
		routes: serverRoutes,
		proxy:  &gatewayReverseProxy,
	}

	rateLimiter := RateLimiter{
		clients:    make(map[string]*ClientBucket),
		maxTokens:  5,
		refillRate: 1,
		next:       &gatewayRouter,
	}

	s := &http.Server{
		Addr:           ":8080",
		Handler:        &rateLimiter,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	s.ListenAndServe()
}
