package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
)

var (
	routes      map[string][]*Backend
	counters    map[string]*uint64
	routesMutex sync.RWMutex // Protects our maps during live reloads
)

// 1. Function to safely load configuration from JSON
func loadConfig() {
	file, err := os.ReadFile("config.json")
	if err != nil {
		log.Printf("⚠️ Error reading config.json: %v", err)
		return
	}

	var tempConfig map[string][]string
	if err := json.Unmarshal(file, &tempConfig); err != nil {
		log.Printf("⚠️ JSON format error: %v", err)
		return
	}

	// Safely lock the maps before updating them to prevent crashes
	routesMutex.Lock()
	defer routesMutex.Unlock()

	routes = make(map[string][]*Backend)
	counters = make(map[string]*uint64)

	for prefix, urls := range tempConfig {
		counters[prefix] = new(uint64)
		for _, u := range urls {
			routes[prefix] = append(routes[prefix], &Backend{URL: u, Alive: true})
		}
	}
	log.Println("🔄 Routing table dynamically loaded from config.json!")
}

// 2. Watch for file saves using fsnotify
func watchConfig() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	watcher.Add("config.json")

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// If the event is a Write (Save), hot-reload the config!
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println("📝 Detected save in config.json. Hot-reloading...")
				loadConfig()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("Watcher error:", err)
		}
	}
}

func main() {
	// Load initial routes and start watching the file in the background
	loadConfig()
	go watchConfig()

	// Start health polling for the initial cluster
	var allBackends []*Backend
	routesMutex.RLock()
	for _, cluster := range routes {
		allBackends = append(allBackends, cluster...)
	}
	routesMutex.RUnlock()
	go activeHealthPolling(allBackends)

	// 3. Upgraded Director
	director := func(req *http.Request) {
		routesMutex.RLock() // Lock for reading so we don't read while it's reloading

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
		routesMutex.RUnlock() // Unlock immediately after finding the cluster

		if targetCluster == nil {
			log.Printf("⚠️ No route found for path: %s", req.URL.Path)
			return
		}

		var targetURL string
		clusterSize := uint64(len(targetCluster))

		for i := uint64(0); i < clusterSize; i++ {
			nextIndex := atomic.AddUint64(activeCounter, 1) % clusterSize
			backend := targetCluster[nextIndex]

			if backend.IsAlive() {
				targetURL = backend.URL
				break
			}
		}

		if targetURL == "" {
			log.Printf("🚨 CRITICAL: All servers for %s are DOWN!", matchedPrefix)
			return
		}

		target, err := url.Parse(targetURL)
		if err != nil {
			return
		}

		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host

		log.Printf("🔀 Routed [%s] request to: %s", matchedPrefix, target.Host)
	}

	proxy := &httputil.ReverseProxy{Director: director}
	log.Println("🚀 API Gateway running on http://localhost:8080 (Hot-Reloading Active)")

	err := http.ListenAndServe(":8080", loggingMiddleware(rateLimitMiddleware(proxy.ServeHTTP)))
	if err != nil {
		log.Fatal(err)
	}
}
