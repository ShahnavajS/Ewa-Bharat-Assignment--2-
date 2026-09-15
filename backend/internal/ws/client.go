package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ─────────────────────────────────────────────────────────────────────────────
// Tuning constants
// ─────────────────────────────────────────────────────────────────────────────

const (
	// writeWait is the max time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// pongWait is the max time we wait for a pong from the peer.
	pongWait = 60 * time.Second

	// pingPeriod is how often we send a ping. Must be < pongWait.
	pingPeriod = 30 * time.Second

	// maxMessageSize is the maximum message size from a client (64 KB).
	maxMessageSize = 64 * 1024

	// sendBufferSize is the bounded outgoing channel size per client.
	// If the client cannot keep up, it will be disconnected.
	sendBufferSize = 64
)

// ─────────────────────────────────────────────────────────────────────────────
// Client represents a single WebSocket connection.
// ─────────────────────────────────────────────────────────────────────────────

// Client is a middleman between a WebSocket connection and the Hub.
type Client struct {
	hub  *Hub
	conn *websocket.Conn

	// send is a bounded channel of outbound messages. If the channel is full,
	// the hub considers the client slow and disconnects it.
	send chan []byte

	// subscribedWindowID is the window this client is subscribed to (0 = none).
	subscribedWindowID int

	// mu protects subscribedWindowID and closed flag.
	mu sync.RWMutex

	// closed is set to true when the client's send channel is closed.
	closed bool

	// closeOnce ensures the close sequence runs exactly once.
	closeOnce sync.Once
}

// newClient creates a new Client attached to the given hub and connection.
func newClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, sendBufferSize),
	}
}

// WindowID returns the currently subscribed window ID (0 if unsubscribed).
func (c *Client) WindowID() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.subscribedWindowID
}

// SetWindowID sets the subscribed window ID.
func (c *Client) SetWindowID(id int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscribedWindowID = id
}

// IsClosed returns whether this client connection has been closed.
func (c *Client) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// SendMessage serializes msg to JSON and sends it to the client's outbound
// channel. Returns false if the channel is full (slow client) or client is closed.
func (c *Client) SendMessage(msg *OutboundMessage) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("ws: failed to marshal outbound message", "error", err)
		return false
	}
	return c.SendRaw(data)
}

// SendRaw sends pre-serialized bytes. Returns false if the channel is full or client is closed.
func (c *Client) SendRaw(data []byte) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return false
	}

	select {
	case c.send <- data:
		return true
	default:
		// Channel full → slow client, signal disconnect.
		return false
	}
}

// Close initiates a clean shutdown of this client exactly once.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		close(c.send)
		c.mu.Unlock()
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// readPump — runs in its own goroutine per client
// ─────────────────────────────────────────────────────────────────────────────

// readPump reads messages from the WebSocket connection and dispatches them
// to the hub. It sets read limits, deadlines, and pong handlers.
// When readPump exits, it triggers client unregistration.
func (c *Client) readPump() {
	defer func() {
		c.hub.UnregisterClient(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
				websocket.CloseNoStatusReceived,
			) {
				slog.Warn("ws: unexpected close", "error", err)
			}
			return
		}

		// Parse the inbound message
		var inMsg InboundMessage
		if err := json.Unmarshal(message, &inMsg); err != nil {
			c.SendMessage(NewErrorMessage("INVALID_JSON", "Malformed JSON message"))
			continue
		}

		c.hub.handleInbound(c, &inMsg)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// writePump — runs in its own goroutine per client
// ─────────────────────────────────────────────────────────────────────────────

// writePump drains the send channel and writes messages to the WebSocket
// connection. It also sends periodic pings.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel closed → hub has unregistered us.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				return
			}
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
