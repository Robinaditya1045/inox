package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter_HealthCheckEndpoint(t *testing.T) {
	// Initialize router passing nil for all handlers and DB connections
	router := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK for healthz probe, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode healthz response json: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp["status"])
	}
	if resp["database"] != "ok" {
		t.Errorf("expected database 'ok' when no pool configured, got '%s'", resp["database"])
	}
	if resp["redis"] != "ok" {
		t.Errorf("expected redis 'ok' when no client configured, got '%s'", resp["redis"])
	}
}
