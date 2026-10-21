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

type GatewayRouter struct {
	mu     sync.RWMutex
	routes map[string]*url.URL
	proxy  *httputil.ReverseProxy
}

// checks if the requested path exists in the Gateway's routes
// if so, move to the proxy. Otherwise return 404 status
func (g *GatewayRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.RLock()

	var matchedPrefix string
	var targetURL *url.URL
	found := false

	for prefix, target := range g.routes {
		if matchesPrefix(r.URL.Path, prefix) && len(prefix) > len(matchedPrefix) {
			matchedPrefix = prefix
			targetURL = target
			found = true
		}
	}

	g.mu.RUnlock()

	if !found {
		http.Error(w, "Route Not Found", http.StatusNotFound)
		return
	}

	ctx := context.WithValue(r.Context(), targetKey, targetURL)
	g.proxy.ServeHTTP(w, r.WithContext(ctx))
}

func matchesPrefix(path, prefix string) bool {
	prefix = strings.TrimSuffix(prefix, "/")

	if prefix == "" {
		return strings.HasPrefix(path, "/")
	}

	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
