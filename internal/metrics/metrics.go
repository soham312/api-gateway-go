// Package metrics provides a minimal, dependency-free /metrics endpoint in
// Prometheus text exposition format: total requests, requests by response
// status code, and per-backend circuit breaker state.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
)

type Metrics struct {
	totalRequests uint64

	mu             sync.Mutex
	requestsByCode map[int]uint64
}

func New() *Metrics {
	return &Metrics{requestsByCode: make(map[int]uint64)}
}

func (m *Metrics) Observe(statusCode int) {
	atomic.AddUint64(&m.totalRequests, 1)
	m.mu.Lock()
	m.requestsByCode[statusCode]++
	m.mu.Unlock()
}

// Middleware records the outcome of every request that passes through it.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		m.Observe(sw.status)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Handler renders the current counters. backendStates, if non-nil, is
// called on every scrape to report each backend's circuit breaker state
// (0 = closed/healthy, 1 = open/down, 2 = half-open/testing).
func (m *Metrics) Handler(backendStates func() map[string]int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		fmt.Fprintln(w, "# HELP gateway_requests_total Total HTTP requests processed by the gateway.")
		fmt.Fprintln(w, "# TYPE gateway_requests_total counter")
		fmt.Fprintf(w, "gateway_requests_total %d\n", atomic.LoadUint64(&m.totalRequests))

		m.mu.Lock()
		codes := make([]int, 0, len(m.requestsByCode))
		counts := make(map[int]uint64, len(m.requestsByCode))
		for code, count := range m.requestsByCode {
			codes = append(codes, code)
			counts[code] = count
		}
		m.mu.Unlock()
		sort.Ints(codes)

		fmt.Fprintln(w, "# HELP gateway_requests_by_status_total HTTP requests by response status code.")
		fmt.Fprintln(w, "# TYPE gateway_requests_by_status_total counter")
		for _, code := range codes {
			fmt.Fprintf(w, "gateway_requests_by_status_total{code=\"%d\"} %d\n", code, counts[code])
		}

		if backendStates == nil {
			return
		}
		states := backendStates()
		urls := make([]string, 0, len(states))
		for url := range states {
			urls = append(urls, url)
		}
		sort.Strings(urls)

		fmt.Fprintln(w, "# HELP gateway_backend_circuit_state Circuit breaker state per backend (0=closed/healthy, 1=open/down, 2=half-open/testing).")
		fmt.Fprintln(w, "# TYPE gateway_backend_circuit_state gauge")
		for _, url := range urls {
			fmt.Fprintf(w, "gateway_backend_circuit_state{backend=%q} %d\n", url, states[url])
		}
	})
}
