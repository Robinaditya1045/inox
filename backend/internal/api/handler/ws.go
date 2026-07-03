package handler

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CheckOrigin prevents Cross-Site WebSocket Hijacking (CSWSH).
	// In local development we allow all origins; in production compare r.Header.Get("Origin") against allowed hosts.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSHandler manages HTTP protocol upgrade requests for real-time room sessions.
type WSHandler struct {
	hub *ws.Hub
}

// NewWSHandler constructs a new WebSocket upgrade controller.
func NewWSHandler(hub *ws.Hub) *WSHandler {
	return &WSHandler{hub: hub}
}

// ServeWS intercepts GET requests to /api/v1/rooms/{id}/ws, verifies authentication and room membership
// via context, and upgrades the connection to a bi-directional WebSocket socket.
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized websocket upgrade attempt")
		return
	}

	roomID := r.PathValue("id")
	if roomID == "" {
		respond.WriteError(w, http.StatusBadRequest, "missing room id parameter")
		return
	}

	// Upgrade HTTP handshake to WebSocket TCP connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket protocol upgrade failed", "room_id", roomID, "user_id", session.UserID, "error", err)
		return
	}

	client := &ws.Client{
		Hub:      h.hub,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		RoomID:   roomID,
		UserID:   session.UserID,
		Username: session.Username,
	}

	// Register client connection with central Hub dispatcher
	h.hub.Register <- client

	// Start independent concurrent pumps for reading and writing frames
	go client.WritePump()
	go client.ReadPump()
}
