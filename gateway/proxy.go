package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func NewReverseProxy() *httputil.ReverseProxy {
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

	return &gatewayReverseProxy
}
