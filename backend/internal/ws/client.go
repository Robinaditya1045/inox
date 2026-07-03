package ws

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer (512KB for WebRTC SDP payloads).
	maxMessageSize = 512 * 1024
)

// Client represents a connected browser session inside a specific Watch Party room.
type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	RoomID   string
	UserID   string
	Username string
}

// readPump pumps incoming messages from the WebSocket connection to the Hub.
//
// The application ensures that there is at most one reader on a connection by
// executing all reads from this goroutine.
func (c *Client) ReadPump() {
	if c.Conn == nil {
		return
	}

	defer func() {
		c.Hub.Unregister <- c
		if c.Conn != nil {
			_ = c.Conn.Close()
		}
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("websocket unexpected close error", "user_id", c.UserID, "error", err)
			}
			break
		}

		// Parse incoming raw JSON event
		var evt Event
		if err := json.Unmarshal(message, &evt); err != nil {
			slog.Warn("malformed websocket event received", "user_id", c.UserID, "error", err)
			continue
		}

		// Inject client sender context into event
		evt.SenderID = c.UserID
		evt.SenderName = c.Username
		evt.RoomID = c.RoomID

		// Route event to Hub dispatcher
		c.Hub.Broadcast <- &evt
	}
}

// writePump pumps messages from the Hub to the WebSocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) WritePump() {
	if c.Conn == nil {
		return
	}

	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		if c.Conn != nil {
			_ = c.Conn.Close()
		}
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The Hub closed the channel.
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Add queued chat messages to the current websocket frame
			n := len(c.Send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
