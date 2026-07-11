package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/api"
	"github.com/inox/inox/backend/internal/ws"
)

func TestHealthCheckEndpoint(t *testing.T) {
	router := api.NewRouter(nil, nil, nil, nil, nil, nil, nil, nil)

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
}

func TestHubGracefulShutdownTeardown(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	client := &ws.Client{
		Hub:      hub,
		Send:     make(chan []byte, 10),
		RoomID:   "room-shutdown-1",
		UserID:   "user-test-1",
		Username: "TestUser",
	}

	hub.Register <- client

	// Give registration a moment to process
	time.Sleep(50 * time.Millisecond)

	// Trigger graceful shutdown
	hub.Shutdown()

	// Verify stop channel closed and client unmapped
	select {
	case <-hub.Stop:
		// Expected: stop channel closed cleanly
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for hub.Stop channel to close")
	}
}
