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

	"github.com/soham312/api-gateway-go/internal/health"
	"github.com/soham312/api-gateway-go/internal/router"
)

type backendKey struct{}

type GatewayTransport struct {
	MaxRetries int
	Backoff    time.Duration
}

func (t *GatewayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	
	backend, ok := req.Context().Value(backendKey{}).(*health.Backend)
	if ok && backend != nil {
		backend.RecordConnection()
		defer backend.ReleaseConnection()
	}

	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body.Close()
	}

	for i := 0; i <= t.MaxRetries; i++ {
		if req.Body != nil || bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
		
		resp, err = http.DefaultTransport.RoundTrip(req)
		
		if err == nil && resp.StatusCode < 500 {
			if ok && backend != nil {
				backend.RecordSuccess()
			}
			return resp, err
		}
		
		if ok && backend != nil {
			backend.RecordFailure()
		}
		
		if i < t.MaxRetries {
			log.Printf("⚠️ Request failed, retrying %d/%d after %v", i+1, t.MaxRetries, t.Backoff)
			time.Sleep(t.Backoff)
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
	
	return &httputil.ReverseProxy{
		Director:  director,
		Transport: &GatewayTransport{MaxRetries: 2, Backoff: 500 * time.Millisecond},
	}
}
