package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	// 1. Define the backend target we want to route traffic to
	targetURL := "https://jsonplaceholder.typicode.com"
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Fatal("Error parsing target URL:", err)
	}

	// 2. Create the reverse proxy using Go's standard library
	proxy := httputil.NewSingleHostReverseProxy(target)

	// 3. Modify the request so the target server accepts it
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}

	// 4. Start the server, but wrap the proxy in our new rate limiter!
	log.Println("🚀 API Gateway is running on http://localhost:8080")

	// This is the magic line that connects Phase 2 and Phase 3:
	err = http.ListenAndServe(":8080", rateLimitMiddleware(proxy.ServeHTTP))
	if err != nil {
		log.Fatal("Server error:", err)
	}
}
