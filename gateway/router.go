package main

import (
	"context"
	"encoding/json"
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

// GatewayRouter performs longest-prefix matching against registered routes
// and coordinates per-route circuit breaking before proxying
type GatewayRouter struct {
	mu     sync.RWMutex
	routes map[string]*RouteConfig
	proxy  *httputil.ReverseProxy
}

type RouteStatus struct {
	Target   string
	State    string
	Failures int
}

// checks if the requested path exists in the Gateway's routes
// if so, move to the proxy. Otherwise return 404 status
func (g *GatewayRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/status" {
		g.handleStatus(w)
		return
	}

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

func (g *GatewayRouter) handleStatus(w http.ResponseWriter) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	statusMap := make(map[string]RouteStatus)

	for prefix, route := range g.routes {
		stateStr, failures := route.Breaker.SnapShot()
		statusMap[prefix] = RouteStatus{
			Target:   route.URL.String(),
			State:    stateStr,
			Failures: failures,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statusMap)
}
