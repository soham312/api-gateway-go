package middleware

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	visitors   map[string]*visitor
	mu         sync.Mutex
	rps        float64
	burst      int
	ttl        time.Duration
	trustProxy bool
}

// NewRateLimiter creates a limiter keyed by client IP. trustProxy controls
// whether X-Forwarded-For/X-Real-IP headers are honored: only set this to
// true when the gateway sits behind a proxy/load balancer that overwrites
// those headers, since otherwise any client can spoof them to get a fresh
// rate-limit bucket per request.
func NewRateLimiter(rps float64, burst int, ttl time.Duration, cleanupInterval time.Duration, trustProxy bool) *RateLimiter {
	rl := &RateLimiter{
		visitors:   make(map[string]*visitor),
		rps:        rps,
		burst:      burst,
		ttl:        ttl,
		trustProxy: trustProxy,
	}

	go rl.cleanupLoop(cleanupInterval)
	return rl
}

func (rl *RateLimiter) cleanupLoop(interval time.Duration) {
	for {
		time.Sleep(interval)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > rl.ttl {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func getIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ips := strings.Split(xff, ",")
			return strings.TrimSpace(ips[0])
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}
	ip := r.RemoteAddr
	if colonIdx := strings.LastIndex(ip, ":"); colonIdx != -1 {
		ip = ip[:colonIdx]
	}
	return ip
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r, rl.trustProxy)

		rl.mu.Lock()
		v, exists := rl.visitors[ip]
		if !exists {
			v = &visitor{limiter: rate.NewLimiter(rate.Limit(rl.rps), rl.burst)}
			rl.visitors[ip] = v
		}
		v.lastSeen = time.Now()
		rl.mu.Unlock()

		if !v.limiter.Allow() {
			log.Printf("🚫 Blocked spam from: %s", ip)
			http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
