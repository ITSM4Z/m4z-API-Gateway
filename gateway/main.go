package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

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
