package main

import (
	"log"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// A dictionary to hold a separate rate limiter for each individual IP address
var visitors = make(map[string]*rate.Limiter)
var mu sync.Mutex

// getVisitor finds the rate limiter for an IP, or creates a new one
func getVisitor(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	limiter, exists := visitors[ip]
	if !exists {
		// Allow 2 requests per second, with a maximum burst of 5 requests
		limiter = rate.NewLimiter(2, 5)
		visitors[ip] = limiter
	}

	return limiter
}

// rateLimitMiddleware is our bouncer. It checks the limit before letting traffic through.
func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the visitor's IP address
		ip := r.RemoteAddr
		limiter := getVisitor(ip)

		// If they exceed the limit, block them
		if !limiter.Allow() {
			log.Printf("🚫 Blocked spam from: %s", ip)
			http.Error(w, "429 Too Many Requests - Slow Down!", http.StatusTooManyRequests)
			return
		}

		// If they are safe, let them through to the reverse proxy
		next.ServeHTTP(w, r)
	}
}
