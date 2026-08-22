package main

import (
	"log"
	"net/http"
	"sync"
	"time"
)

// 1. The State Machine for each backend server
type Backend struct {
	URL   string
	Alive bool
	mux   sync.RWMutex // Prevents race conditions when reading/writing health status
}

// 2. Safely updates the server's health state (Circuit Breaker Switch)
func (b *Backend) SetStatus(status bool) {
	b.mux.Lock()
	b.Alive = status
	b.mux.Unlock()
}

// 3. Safely checks if the server is healthy before routing traffic
func (b *Backend) IsAlive() bool {
	b.mux.RLock()
	alive := b.Alive
	b.mux.RUnlock()
	return alive
}

// 4. Active Health Polling and Retry Loop logic
func activeHealthPolling(backends []*Backend) {
	for {
		for _, b := range backends {
			// Ping the server with a 2-second timeout
			client := http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get(b.URL)

			if err != nil || resp.StatusCode >= 500 {
				if b.IsAlive() {
					b.SetStatus(false)
					log.Printf("🔴 CIRCUIT TRIPPED: %s is DOWN! Halting traffic.", b.URL)
				}
			} else {
				if !b.IsAlive() {
					b.SetStatus(true)
					log.Printf("🟢 CIRCUIT CLOSED: %s is recovering and HEALTHY.", b.URL)
				}
			}
		}
		// Wait 10 seconds before polling again
		time.Sleep(10 * time.Second)
	}
}
