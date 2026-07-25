package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_WhitelistedOrigin(t *testing.T) {
	SetAllowedOrigins("http://localhost:5173,https://inox.app")

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rooms", nil)
	req.Header.Set("Origin", "https://inox.app")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://inox.app" {
		t.Errorf("expected Access-Control-Allow-Origin https://inox.app, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials true, got %q", got)
	}
}

func TestCORS_NonWhitelistedOrigin(t *testing.T) {
	SetAllowedOrigins("http://localhost:5173,https://inox.app")

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rooms", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from handler when non-OPTIONS, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected empty Access-Control-Allow-Origin for non-whitelisted origin, got %q", got)
	}
}

func TestCORS_PreflightStrictEnforcement(t *testing.T) {
	SetAllowedOrigins("https://inox.app")

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for OPTIONS preflight")
	}))

	// 1. Whitelisted preflight -> 200 OK
	reqValid := httptest.NewRequest(http.MethodOptions, "/api/v1/rooms", nil)
	reqValid.Header.Set("Origin", "https://inox.app")
	recValid := httptest.NewRecorder()

	handler.ServeHTTP(recValid, reqValid)
	if recValid.Code != http.StatusOK {
		t.Errorf("expected status 200 for valid preflight, got %d", recValid.Code)
	}

	// 2. Non-whitelisted preflight -> 403 Forbidden
	reqInvalid := httptest.NewRequest(http.MethodOptions, "/api/v1/rooms", nil)
	reqInvalid.Header.Set("Origin", "https://evil.com")
	recInvalid := httptest.NewRecorder()

	handler.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden for non-whitelisted preflight, got %d", recInvalid.Code)
	}
}
