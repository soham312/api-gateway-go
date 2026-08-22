package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
)

// 1. Create a list of different backend servers (we are using 3 public testing APIs)
var backends = []string{
	"https://jsonplaceholder.typicode.com",
	"https://dummyjson.com",
	"https://api.restful-api.dev",
}

// A counter to keep track of whose turn it is
var requestCounter uint64 = 0

func main() {
	// 2. We write a custom Director function to pick a server dynamically
	director := func(req *http.Request) {
		// Safely add 1 to the counter and divide by 3 to get the next index (0, 1, or 2)
		nextIndex := atomic.AddUint64(&requestCounter, 1) % uint64(len(backends))
		targetURL := backends[nextIndex]

		target, err := url.Parse(targetURL)
		if err != nil {
			log.Println("Error parsing target:", err)
			return
		}

		// Route the traffic to the chosen server
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host

		// Print a message in the terminal so we can see the load balancer working!
		log.Printf("⚖️ Load Balancer routed request to: %s", target.Host)
	}

	// 3. Create the proxy using our custom load-balancing director
	proxy := &httputil.ReverseProxy{Director: director}

	log.Println("🚀 API Gateway running on http://localhost:8080 (Load Balancing Active)")

	// 4. Start the server (our Phase 3 rate limiter is still protecting the front door!)
	err := http.ListenAndServe(":8080", loggingMiddleware(rateLimitMiddleware(proxy.ServeHTTP)))
	if err != nil {
		log.Fatal("Server error:", err)
	}
}
