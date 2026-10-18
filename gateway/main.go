package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func main() {
	backendStringURL := "http://localhost:9000"

	backendURL, err := url.Parse(backendStringURL)
	if err != nil {
		return
	}

	// The pool of reusable TCP Pipes the Gateway will be using when talking to :9000
	gatewayTransport := http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}
	gatewayReverseProxy := httputil.ReverseProxy{
		Transport: &gatewayTransport,
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetXForwarded()
			r.SetURL(backendURL)
		},
	}

	s := &http.Server{
		Addr:           ":8080",
		Handler:        &gatewayReverseProxy,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	s.ListenAndServe()
}
