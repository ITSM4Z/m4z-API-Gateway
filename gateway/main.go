package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

func main() {
	rawRoutes := map[string]string{
		"/users":  "http://localhost:9001",
		"/orders": "http://localhost:9002",
	}

	serverRoutes := make(map[string]*RouteConfig)

	for prefix, targetStr := range rawRoutes {
		parsedURL, err := url.Parse(targetStr)

		if err != nil {
			fmt.Printf("An Error occured while parsing (%s): %v", targetStr, err)
			return
		}

		serverRoutes[prefix] = &RouteConfig{
			URL: parsedURL,
			Breaker: &CircuitBreaker{
				failureThreshold: 3,
				cooldown:         10 * time.Second,
				state:            StateClosed,
			},
		}
	}

	proxy := NewReverseProxy()

	gatewayRouter := GatewayRouter{
		routes: serverRoutes,
		proxy:  proxy,
	}

	rateLimiter := &RateLimiter{
		clients:    make(map[string]*ClientBucket),
		maxTokens:  5,
		refillRate: 1,
	}

	stopCleanup := make(chan struct{})
	go rateLimiter.cleanupLoop(stopCleanup)
	defer close(stopCleanup)

	handler := rateLimiter.Middleware(&gatewayRouter)

	s := &http.Server{
		Addr:           ":8080",
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
