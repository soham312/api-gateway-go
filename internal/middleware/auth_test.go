package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func signToken(t *testing.T, secret string, expired bool) string {
	t.Helper()
	exp := time.Now().Add(time.Hour)
	if expired {
		exp = time.Now().Add(-time.Hour)
	}
	claims := jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(exp)}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return signed
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	auth := NewJWTAuth("secret")
	handler := auth.Middleware(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing header, got %d", rec.Code)
	}
}

func TestJWTAuth_MalformedHeader(t *testing.T) {
	auth := NewJWTAuth("secret")
	handler := auth.Middleware(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "NotBearer sometoken")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for malformed header, got %d", rec.Code)
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	auth := NewJWTAuth("secret")
	handler := auth.Middleware(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", rec.Code)
	}
}

func TestJWTAuth_WrongSecret(t *testing.T) {
	auth := NewJWTAuth("correct-secret")
	handler := auth.Middleware(okHandler())

	token := signToken(t, "wrong-secret", false)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for token signed with wrong secret, got %d", rec.Code)
	}
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	auth := NewJWTAuth("secret")
	handler := auth.Middleware(okHandler())

	token := signToken(t, "secret", true)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", rec.Code)
	}
}

func TestJWTAuth_RejectsNoneAlgorithm(t *testing.T) {
	auth := NewJWTAuth("secret")
	handler := auth.Middleware(okHandler())

	// Forge a token using alg=none (no signature at all) - a classic JWT
	// bypass when the verifier doesn't pin the expected signing method.
	claims := jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	forged, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to forge none-alg token: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+forged)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for alg=none forged token, got %d", rec.Code)
	}
}

func TestJWTAuth_ValidToken(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	auth := NewJWTAuth("secret")
	handler := auth.Middleware(next)

	token := signToken(t, "secret", false)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Errorf("expected next handler to be called for valid token")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid token, got %d", rec.Code)
	}
}
