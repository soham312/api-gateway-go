package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
)

// 1. Dynamic routing table mapping URL prefixes to target clusters
var routes = map[string][]string{
	"/users": {
		"https://jsonplaceholder.typicode.com",
		"https://dummyjson.com",
	},
	"/products": {
		"https://api.restful-api.dev",
	},
}

// 2. A map of counters so each route has its own independent load balancer
var counters = map[string]*uint64{
	"/users":    new(uint64),
	"/products": new(uint64),
}

func main() {
	// 3. Upgraded Director for Longest-Prefix Routing
	director := func(req *http.Request) {
		var targetCluster []string
		var activeCounter *uint64
		var matchedPrefix string

		// Scan the incoming URL to see which route it matches
		for prefix, backends := range routes {
			if strings.HasPrefix(req.URL.Path, prefix) {
				targetCluster = backends
				activeCounter = counters[prefix]
				matchedPrefix = prefix
				break
			}
		}

		// If the user asks for a path we don't have, drop the request
		if targetCluster == nil {
			log.Printf("⚠️ No route found for path: %s", req.URL.Path)
			return
		}

		// Apply Round-Robin Load Balancing STRICTLY to the matched cluster
		nextIndex := atomic.AddUint64(activeCounter, 1) % uint64(len(targetCluster))
		targetURL := targetCluster[nextIndex]

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

	// 4. Create the proxy using our custom load-balancing director
	proxy := &httputil.ReverseProxy{Director: director}

	log.Println("🚀 API Gateway running on http://localhost:8080 (Dynamic Routing Active)")

	// 5. Start the server with Logging -> Rate Limiter -> Load Balancer -> Backend
	err := http.ListenAndServe(":8080", loggingMiddleware(rateLimitMiddleware(proxy.ServeHTTP)))
	if err != nil {
		log.Fatal("Server error:", err)
	}
}
