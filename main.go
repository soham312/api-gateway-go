package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
)

// 1. Upgraded routing table using our new Backend objects for Fault Tolerance
var routes = map[string][]*Backend{
	"/users": {
		{URL: "https://jsonplaceholder.typicode.com", Alive: true},
		{URL: "https://dummyjson.com", Alive: true},
	},
	"/products": {
		{URL: "https://api.restful-api.dev", Alive: true},
	},
}

var counters = map[string]*uint64{
	"/users":    new(uint64),
	"/products": new(uint64),
}

func main() {
	// 2. Start the Active Health Polling in the background
	// Gather all backends into one list to pass to the health checker
	var allBackends []*Backend
	for _, cluster := range routes {
		allBackends = append(allBackends, cluster...)
	}

	// The 'go' keyword starts this loop as an independent background task!
	go activeHealthPolling(allBackends)

	// 3. Upgraded Director with State-Machine Circuit Breaker logic
	director := func(req *http.Request) {
		var targetCluster []*Backend
		var activeCounter *uint64
		var matchedPrefix string

		for prefix, backends := range routes {
			if strings.HasPrefix(req.URL.Path, prefix) {
				targetCluster = backends
				activeCounter = counters[prefix]
				matchedPrefix = prefix
				break
			}
		}

		if targetCluster == nil {
			log.Printf("⚠️ No route found for path: %s", req.URL.Path)
			return
		}

		// Circuit Breaker: Find the next HEALTHY server
		var targetURL string
		clusterSize := uint64(len(targetCluster))

		for i := uint64(0); i < clusterSize; i++ {
			nextIndex := atomic.AddUint64(activeCounter, 1) % clusterSize
			backend := targetCluster[nextIndex]

			// Only route traffic if the circuit is closed (server is Alive)
			if backend.IsAlive() {
				targetURL = backend.URL
				break
			}
		}

		// If the entire cluster crashed
		if targetURL == "" {
			log.Printf("🚨 CRITICAL: All servers for %s are DOWN!", matchedPrefix)
			return
		}

		target, err := url.Parse(targetURL)
		if err != nil {
			log.Println("Error parsing target:", err)
			return
		}

		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host

		log.Printf("🔀 Routed [%s] request to: %s", matchedPrefix, target.Host)
	}

	proxy := &httputil.ReverseProxy{Director: director}

	log.Println("🚀 API Gateway running on http://localhost:8080 (Circuit Breakers Active)")

	err := http.ListenAndServe(":8080", loggingMiddleware(rateLimitMiddleware(proxy.ServeHTTP)))
	if err != nil {
		log.Fatal("Server error:", err)
	}
}
