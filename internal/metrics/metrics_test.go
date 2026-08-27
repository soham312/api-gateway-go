package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetrics_MiddlewareRecordsStatusCodes(t *testing.T) {
	m := New()
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	rec := httptest.NewRecorder()
	m.Handler(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	if !strings.Contains(body, "gateway_requests_total 3") {
		t.Errorf("expected total requests of 3, got body:\n%s", body)
	}
	if !strings.Contains(body, `gateway_requests_by_status_total{code="418"} 3`) {
		t.Errorf("expected status 418 count of 3, got body:\n%s", body)
	}
}

func TestMetrics_MiddlewareDefaultsTo200WhenWriteHeaderNotCalled(t *testing.T) {
	m := New()
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	out := httptest.NewRecorder()
	m.Handler(nil).ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if !strings.Contains(out.Body.String(), `gateway_requests_by_status_total{code="200"} 1`) {
		t.Errorf("expected implicit 200 to be recorded, got body:\n%s", out.Body.String())
	}
}

func TestMetrics_HandlerReportsBackendStates(t *testing.T) {
	m := New()
	rec := httptest.NewRecorder()
	m.Handler(func() map[string]int {
		return map[string]int{"http://backend-a": 0, "http://backend-b": 1}
	}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := rec.Body.String()
	if !strings.Contains(body, `gateway_backend_circuit_state{backend="http://backend-a"} 0`) {
		t.Errorf("expected backend-a state, got body:\n%s", body)
	}
	if !strings.Contains(body, `gateway_backend_circuit_state{backend="http://backend-b"} 1`) {
		t.Errorf("expected backend-b state, got body:\n%s", body)
	}
}

func TestMetrics_HandlerOmitsBackendSectionWhenNilFunc(t *testing.T) {
	m := New()
	rec := httptest.NewRecorder()
	m.Handler(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if strings.Contains(rec.Body.String(), "gateway_backend_circuit_state") {
		t.Errorf("did not expect backend state section, got body:\n%s", rec.Body.String())
	}
}
