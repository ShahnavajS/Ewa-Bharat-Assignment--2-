package ws

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

// ServeWS handles an HTTP request and upgrades it to a WebSocket connection.
// It is meant to be called from the Gin router:
//
//	r.GET("/api/ws", func(c *gin.Context) { hub.ServeWS(c.Writer, c.Request) })
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	if h.IsStopped() {
		http.Error(w, "WebSocket hub is shutting down", http.StatusServiceUnavailable)
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(req *http.Request) bool {
			return h.originAllowed(req.Header.Get("Origin"))
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws: upgrade failed", "error", err)
		return
	}

	client := newClient(h, conn)
	h.RegisterClient(client)

	// Start the client's read and write pumps in their own goroutines.
	go client.writePump()
	go client.readPump()
}
