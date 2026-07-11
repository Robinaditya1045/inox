package handler

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/observability"
)

var telemetryUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, enforce origin checks against admin dashboard domain
	},
}

// AdminHandler handles telemetry, observability, and administrative intervention endpoints.
type AdminHandler struct {
	telemetryHub *observability.TelemetryHub
}

// NewAdminHandler initializes the admin telemetry controller.
func NewAdminHandler(hub *observability.TelemetryHub) *AdminHandler {
	return &AdminHandler{
		telemetryHub: hub,
	}
}

// ServePrometheus exports all system metrics in standard Prometheus text exposition format.
// Route: GET /metrics
func (h *AdminHandler) ServePrometheus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(observability.Global().FormatPrometheus()))
}

// ServeTelemetryWS upgrades an HTTP connection to a WebSocket for real-time telemetry streaming.
// Route: GET /api/v1/admin/telemetry/ws
func (h *AdminHandler) ServeTelemetryWS(w http.ResponseWriter, r *http.Request) {
	conn, err := telemetryUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("failed to upgrade admin websocket connection", "error", err)
		return
	}

	h.telemetryHub.Register(conn)

	// Listen for close/disconnect events from the client
	go func() {
		defer func() {
			h.telemetryHub.Unregister(conn)
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

// GetSnapshot returns an immediate JSON snapshot of current system telemetry.
// Route: GET /api/v1/admin/telemetry
func (h *AdminHandler) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshot := h.telemetryHub.GetSnapshot()
	respond.WriteJSON(w, http.StatusOK, snapshot)
}
