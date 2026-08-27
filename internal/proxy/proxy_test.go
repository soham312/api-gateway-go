package proxy

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soham312/api-gateway-go/internal/health"
	"github.com/soham312/api-gateway-go/internal/router"
)

type staticBalancer struct {
	backend *health.Backend
}

func (s *staticBalancer) Next() *health.Backend { return s.backend }

func TestProxy_ForwardsToBackend(t *testing.T) {
	backendSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hello" {
			t.Errorf("expected path /hello, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer backendSrv.Close()

	backend := health.NewBackend(backendSrv.URL, 1)
	r := router.NewRouter([]router.Route{
		{Prefix: "/api", StripPrefix: true, Balancer: &staticBalancer{backend: backend}},
	})
	p := New(r)

	req := httptest.NewRequest("GET", "/api/hello", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", rec.Body.String())
	}
}

func TestProxy_RetriesOn5xxThenSucceeds(t *testing.T) {
	var attempts int32
	backendSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backendSrv.Close()

	backend := health.NewBackend(backendSrv.URL, 1)
	r := router.NewRouter([]router.Route{
		{Prefix: "/", StripPrefix: false, Balancer: &staticBalancer{backend: backend}},
	})
	p := New(r)

	req := httptest.NewRequest("GET", "/anything", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected eventual 200 after retry, got %d", rec.Code)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected exactly 2 attempts, got %d", attempts)
	}
}

func TestProxy_NoRouteMatch_FailsFast(t *testing.T) {
	r := router.NewRouter([]router.Route{
		{Prefix: "/known", StripPrefix: false, Balancer: &staticBalancer{}},
	})
	p := New(r)

	req := httptest.NewRequest("GET", "/unknown", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	p.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	// Director leaves the request untouched, so the proxy attempts to
	// round-trip a relative URL and fails immediately (no backend was ever
	// selected, so retrying with backoff would be pointless).
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 for unroutable request, got %d", rec.Code)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("expected fail-fast (<200ms) for unroutable request, took %v", elapsed)
	}
}

func TestProxy_AllBackendsDown_FailsFast(t *testing.T) {
	r := router.NewRouter([]router.Route{
		{Prefix: "/api", StripPrefix: false, Balancer: &staticBalancer{backend: nil}},
	})
	p := New(r)

	req := httptest.NewRequest("GET", "/api/x", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	p.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 when no backend available, got %d", rec.Code)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("expected fail-fast (<200ms) when no backend available, took %v", elapsed)
	}
}
