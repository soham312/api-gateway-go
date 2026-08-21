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
		// We must explicitly set the Host header for HTTPS targets
		req.Host = target.Host 
	}

	// 4. Create a handler to capture traffic and pass it to the proxy
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Gateway intercepted request for: %s", r.URL.Path)
		proxy.ServeHTTP(w, r)
	})

	// 5. Start the API Gateway on port 8080
	log.Println("🚀 API Gateway is running on http://localhost:8080")
	log.Printf("➡️  Forwarding all traffic to %s", targetURL)
	
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Server error:", err)
	}
}