package ws

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
	"github.com/inox/inox/backend/internal/domain"
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

	// Role and JoinedAt exist so the hub can elect a live sync leader without a
	// database round-trip inside its event loop. Ranking is owner, then moderator,
	// then whoever has been connected longest.
	Role     domain.Role
	JoinedAt time.Time

	// CanLead is cleared when a client reports it cannot produce stream positions,
	// which is the case for players using native HLS instead of hls.js. Such a client
	// would hold the leadership claim and publish nothing.
	CanLead bool
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

			// One event per frame. Packing several newline-separated events into one
			// frame -- as gorilla's chat example does -- hands the browser a body that
			// is not valid JSON, and JSON.parse drops the whole frame. Bursts are
			// routine here (an SDP offer followed by a dozen ICE candidates), so that
			// silently broke exactly the exchanges that matter most.
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
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
