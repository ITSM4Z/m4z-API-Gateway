# Custom API Gateway in Go

A lightweight API Gateway built from scratch in Go. I built this project to move past basic web frameworks and learn how networking primitives, reverse proxies, concurrency, and fault-tolerance patterns work under the hood.

---

## What It Does

This gateway sits between clients and backend services, handling three main responsibilities:
1. **Reverse Proxy & Dynamic Routing:** Forwards incoming requests to different backend services based on the URL path prefix (e.g., `/users` goes to one service, `/orders` to another) while reusing TCP connections.
2. **Rate Limiting:** Protects backend services from being flooded by using a Token Bucket algorithm tracked per client IP.
3. **Circuit Breaking:** Automatically stops sending requests to a backend service if it starts failing repeatedly, failing fast until the service recovers.

---

## Project Structure

```text
├── gateway/
│   ├── main.go            # Entry point for the gateway server
│   ├── router.go          # Handles path prefix matching and /status telemetry
│   ├── proxy.go           # Configures the reverse proxy and transport pool
│   ├── ratelimiter.go     # Token bucket rate limiting middleware + cleanup goroutine
│   └── circuitbreaker.go  # State machine (Closed/Open/Half-Open) & response recorder
├── mockbackend/
│   ├── main.go            # Simple mock server used for testing backend routes
└── run.go                 # Root helper script to spin up everything at once
```

---

## How to Run it

You can run the entire system (the gateway and two mock backend servers) using the helper script at the root:
```bash
go run run.go
```
### Testing the Endpoints
Once it's running on `localhost:8080`:
- Test the Users service:
  ```bash
  curl http://localhost:8080/users
  ```
- Test the Orders service:
  ```bash
  curl http://localhost:8080/orders
  ```
- Test the Rate Limiter:
  Run this command 6 times quickly in a row, the 6th request will be blocked:
  ```bash
  curl http://localhost:8080/users
  ```
- Test the Circuit Breaker:
  - **1.** Trigger 3 errors on the mock backend: `curl http://localhost:8080/users/fail` (do this 3 times).
  - **2.** Check the gateway status: `curl http://localhost:8080/status` (you will see the circuit breaker state change to `OPEN`).
  - **3.** Further requests will immediately return `Service Unavailable` without hitting the dead backend.

---

## What I Learned Building This

* **Concurrency & Mutexes:** Used `sync.RWMutex` in the router so multiple requests can read the route map at the same time without blocking each other, and `sync.Mutex` in the rate limiter and circuit breaker where every request mutates state.
* **Pointers & Shared State:** Passed structs like `CircuitBreaker` and `ClientBucket` by pointer (`*`) so concurrent requests update the same underlying state in memory instead of modifying copies.
* **Go `net/http` Internals:** Built custom middleware using `http.Handler`, passed route data across handlers using `context.Context`, wrapped `http.ResponseWriter` to capture downstream status codes, and tuned `http.Transport` to reuse idle TCP connections.
* **Memory Management:** Added a background goroutine with `time.NewTicker` to periodically clean up inactive IP buckets from the rate limiter map so memory doesn't grow forever.

---

## Known Limitations & Future Improvements

* **Single-Node State:** Right now, rate limiter buckets and circuit breaker states are stored in local memory (`map`). If multiple instances of this gateway were running behind a load balancer, they wouldn't share state. Moving the token bucket state to **Redis** would make it work across multiple nodes.
* **Static Configuration:** Routes are currently defined in `main.go` at startup. A nice next step would be loading routes from a config file (YAML/JSON) or database so routes can be added dynamically without restarting the server.
