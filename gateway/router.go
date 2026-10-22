package main

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

type ctxKey string

const targetKey ctxKey = "target"

type RouteConfig struct {
	URL     *url.URL
	Breaker *CircuitBreaker
}

type GatewayRouter struct {
	mu     sync.RWMutex
	routes map[string]*RouteConfig
	proxy  *httputil.ReverseProxy
}

// checks if the requested path exists in the Gateway's routes
// if so, move to the proxy. Otherwise return 404 status
func (g *GatewayRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.RLock()

	var matchedPrefix string
	var targetRoute *RouteConfig
	found := false

	for prefix, target := range g.routes {
		if matchesPrefix(r.URL.Path, prefix) && len(prefix) > len(matchedPrefix) {
			matchedPrefix = prefix
			targetRoute = target
			found = true
		}
	}

	g.mu.RUnlock()

	if !found {
		http.Error(w, "Route Not Found", http.StatusNotFound)
		return
	}

	if !targetRoute.Breaker.AllowRequest() {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	rec := &statusRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}

	ctx := context.WithValue(r.Context(), targetKey, targetRoute.URL)
	g.proxy.ServeHTTP(rec, r.WithContext(ctx))
	targetRoute.Breaker.RecordResult(rec.statusCode)
}

func matchesPrefix(path, prefix string) bool {
	prefix = strings.TrimSuffix(prefix, "/")

	if prefix == "" {
		return strings.HasPrefix(path, "/")
	}

	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
