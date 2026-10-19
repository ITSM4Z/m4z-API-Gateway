package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ctxKey string

const targetKey ctxKey = "target"

type GatewayRouter struct {
	mu     sync.RWMutex
	routes map[string]*url.URL
	proxy  *httputil.ReverseProxy
}

func (g *GatewayRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.RLock()

	for prefix, targetURL := range g.routes {
		if strings.HasPrefix(r.URL.Path, prefix) {
			ctx := context.WithValue(r.Context(), targetKey, targetURL)
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

	s := &http.Server{
		Addr:           ":8080",
		Handler:        &gatewayRouter,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	s.ListenAndServe()
}
