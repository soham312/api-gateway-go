package main

import (
	"log"
	"net/http"
	"time"
)

// loggingMiddleware acts as a receptionist, writing down the details of every visitor
func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Start a stopwatch when the request arrives
		startTime := time.Now()

		// 2. Let the request pass through to the rate limiter and load balancer
		next.ServeHTTP(w, r)

		// 3. Stop the stopwatch and log the details
		duration := time.Since(startTime)
		log.Printf("📝 LOG: %s request to %s took %v", r.Method, r.URL.Path, duration)
	}
}
