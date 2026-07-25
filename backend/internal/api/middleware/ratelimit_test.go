package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimit_EnforcementAndClientIP(t *testing.T) {
	limiter := RateLimit(2, time.Second)

	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	// Test 1: First request from IP A -> 200 OK
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req1.RemoteAddr = "10.0.0.1:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 for 1st request, got %d", rec1.Code)
	}

	// Test 2: Second request from IP A -> 200 OK
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req2.RemoteAddr = "10.0.0.1:1234"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for 2nd request, got %d", rec2.Code)
	}

	// Test 3: Third request from IP A -> 429 Too Many Requests
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req3.RemoteAddr = "10.0.0.1:1234"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests on 3rd request, got %d", rec3.Code)
	}
	if got := rec3.Header().Get("Retry-After"); got == "" {
		t.Errorf("expected Retry-After header on 429 response, got empty")
	}

	// Test 4: Request from a different IP B -> 200 OK (independent rate limit)
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req4.Header.Set("X-Forwarded-For", "192.168.1.50, 10.0.0.1")
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 for independent IP B via X-Forwarded-For, got %d", rec4.Code)
	}
}
