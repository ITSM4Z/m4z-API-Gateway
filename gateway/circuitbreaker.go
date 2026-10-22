package main

import (
	"net/http"
	"sync"
	"time"
)

type BreakerState int

const (
	StateClosed   BreakerState = iota // 0: Operating normally
	StateHalfOpen                     // 1: Testing recovary
	StateOpen                         // 2: Failing fast
)

// CircuitBreaker implements a thread-safe state machine (Closed, Half-Open, Open)
// that prevents cascading failures when a downstream backend is unhealthy
type CircuitBreaker struct {
	mu               sync.Mutex
	state            BreakerState
	failures         int
	failureThreshold int
	cooldown         time.Duration
	lastFailure      time.Time
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

func (s BreakerState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateHalfOpen:
		return "HALF-OPEN"
	case StateOpen:
		return "OPEN"
	default:
		return "UNKNOWN"
	}
}

// AllowRequest checks if the circuit is Closed or can transition from Open to Half-Open
// after the cooldown period
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateClosed {
		return true
	}
	if cb.state == StateOpen {
		if time.Since(cb.lastFailure) > cb.cooldown {
			cb.state = StateHalfOpen
			return true
		} else {
			return false
		}
	}
	if cb.state == StateHalfOpen {
		return true
	}

	return false
}

// RecordResult updates the breaker state based on the downstream HTTP status code
// switches to Open when consecutive 5xx failures reach failureThreshold
func (cb *CircuitBreaker) RecordResult(statusCode int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if statusCode >= 500 {
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.failures >= cb.failureThreshold || cb.state == StateHalfOpen {
			cb.state = StateOpen
		}
	} else {
		cb.failures = 0
		cb.state = StateClosed
	}
}

func (cb *CircuitBreaker) SnapShot() (string, int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.state.String(), cb.failures
}
