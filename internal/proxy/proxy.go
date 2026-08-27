package proxy

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/soham312/api-gateway-go/internal/config"
	"github.com/soham312/api-gateway-go/internal/health"
	"github.com/soham312/api-gateway-go/internal/router"
)

type backendKey struct{}

type GatewayTransport struct {
	MaxRetries int
	Backoff    time.Duration
	Transport  http.RoundTripper
}

func (t *GatewayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	backend, ok := req.Context().Value(backendKey{}).(*health.Backend)
	routed := ok && backend != nil
	if routed {
		backend.RecordConnection()
		defer backend.ReleaseConnection()
	}

	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body.Close()
	}

	cfg := config.Get()
	maxRetries := t.MaxRetries
	backoff := t.Backoff
	maxDelay := t.Backoff // for simplicity if maxDelay not set
	if cfg != nil {
		if cfg.RetryMaxAttempts > 0 {
			maxRetries = cfg.RetryMaxAttempts
		}
		if cfg.RetryBaseDelay != "" {
			if d, err := time.ParseDuration(cfg.RetryBaseDelay); err == nil {
				backoff = d
			}
		}
		if cfg.RetryMaxDelay != "" {
			if d, err := time.ParseDuration(cfg.RetryMaxDelay); err == nil {
				maxDelay = d
			}
		}
	}
	if !routed {
		// No route matched or every backend was down before we ever made a
		// request: retrying will not change that, so fail fast instead of
		// paying the full backoff schedule.
		maxRetries = 0
	}

	for i := 0; i <= maxRetries; i++ {
		if req.Body != nil || bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		resp, err = t.Transport.RoundTrip(req)

		if err == nil && resp.StatusCode < 500 {
			if routed {
				backend.RecordSuccess()
			}
			return resp, err
		}

		if routed {
			backend.RecordFailure()
		}

		if i < maxRetries {
			log.Printf("⚠️ Request failed, retrying %d/%d after %v", i+1, maxRetries, backoff)
			time.Sleep(backoff)

			// Simple exponential backoff
			backoff *= 2
			if backoff > maxDelay {
				backoff = maxDelay
			}
		}
	}
	return resp, err
}

func New(r *router.Router) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		route, newPath := r.Match(req.URL.Path)
		if route == nil {
			log.Printf("⚠️ No route found for path: %s", req.URL.Path)
			return
		}

		backend := route.Balancer.Next()
		if backend == nil {
			log.Printf("🚨 CRITICAL: All servers for %s are DOWN!", route.Prefix)
			return
		}

		*req = *req.WithContext(context.WithValue(req.Context(), backendKey{}, backend))

		target, _ := url.Parse(backend.URL)
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = newPath
		req.Host = target.Host
	}

	// Go's http.DefaultTransport caps idle connections at 2 per host, which
	// is fine for a single client talking to many hosts but wrong for a
	// gateway: many concurrent client requests fan in through this one
	// process to a handful of backend hosts. Without a larger per-host pool,
	// load beyond 2 concurrent requests to the same backend forces a fresh
	// TCP (and TLS, for https backends) handshake per request instead of
	// reusing a connection.
	backendTransport := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	return &httputil.ReverseProxy{
		Director:  director,
		Transport: &GatewayTransport{MaxRetries: 2, Backoff: 500 * time.Millisecond, Transport: backendTransport},
	}
}
