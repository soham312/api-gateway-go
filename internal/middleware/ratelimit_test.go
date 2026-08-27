package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetIP_TrustProxyUsesXForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	if got := getIP(req, true); got != "1.2.3.4" {
		t.Errorf("expected first XFF entry, got %q", got)
	}
}

func TestGetIP_TrustProxyUsesXRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "9.9.9.9")
	if got := getIP(req, true); got != "9.9.9.9" {
		t.Errorf("expected X-Real-IP value, got %q", got)
	}
}

func TestGetIP_RemoteAddrFallback(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	if got := getIP(req, true); got != "10.0.0.1" {
		t.Errorf("expected remote addr without port, got %q", got)
	}
}

func TestGetIP_UntrustedProxyIgnoresSpoofedHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "5.6.7.8")
	req.RemoteAddr = "10.0.0.1:54321"

	if got := getIP(req, false); got != "10.0.0.1" {
		t.Errorf("expected spoofed headers to be ignored when trustProxy=false, got %q", got)
	}
}

func TestRateLimiter_UntrustedProxyCannotBypassViaSpoofedHeader(t *testing.T) {
	rl := NewRateLimiter(1, 1, time.Minute, time.Minute, false)
	handler := rl.Middleware(okHandler())

	// Same underlying connection (RemoteAddr), attacker varies X-Forwarded-For
	// on each request to try to get a fresh rate-limit bucket every time.
	makeReq := func(spoofedIP string) *http.Request {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "1.1.1.1:1111"
		req.Header.Set("X-Forwarded-For", spoofedIP)
		return req
	}

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, makeReq("2.2.2.2"))
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request to succeed, got %d", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, makeReq("3.3.3.3"))
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected spoofed X-Forwarded-For to be ignored and rate limit enforced, got %d", rec2.Code)
	}
}

func TestRateLimiter_AllowsBurstThenBlocks(t *testing.T) {
	rl := NewRateLimiter(1, 2, time.Minute, time.Minute, false)
	handler := rl.Middleware(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.1.1.1:1111"

	// burst of 2 should succeed immediately
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 within burst, got %d", i, rec.Code)
		}
	}

	// third immediate request should be rate limited
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after burst exhausted, got %d", rec.Code)
	}
}

func TestRateLimiter_SeparateIPsIndependent(t *testing.T) {
	rl := NewRateLimiter(1, 1, time.Minute, time.Minute, false)
	handler := rl.Middleware(okHandler())

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "1.1.1.1:1111"
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "2.2.2.2:2222"

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 for first IP, got %d", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200 for second (independent) IP, got %d", rec2.Code)
	}
}
