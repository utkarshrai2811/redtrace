package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/utkarshrai2811/redtrace/internal/storage"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	clientSendBuf  = 256
	maxMessageSize = 1024
)

// envelope is the JSON frame pushed over the traffic WebSocket.
type envelope struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// InterceptState is broadcast whenever the interception queue changes.
type InterceptState struct {
	Enabled            bool       `json:"enabled"`
	InterceptResponses bool       `json:"interceptResponses"`
	Queue              []HeldItem `json:"queue"`
	Count              int        `json:"count"`
}

// Hub fans out live events (captured traffic, interception updates) to all
// connected WebSocket clients. It is safe for concurrent use.
type Hub struct {
	mu      sync.Mutex
	clients map[*wsClient]struct{}
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

// NewHub returns an empty Hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[*wsClient]struct{})}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// The API binds to loopback by default; allow any origin so the local UI
	// (possibly on a different dev port) can connect.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ServeWS upgrades the connection and registers it with the hub.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &wsClient{conn: conn, send: make(chan []byte, clientSendBuf)}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go h.writePump(c)
	h.readPump(c)
}

// Broadcast sends a typed event to all connected clients, dropping the message
// for any client whose buffer is full (a slow consumer never blocks the proxy).
func (h *Hub) Broadcast(eventType string, data any) {
	msg, err := json.Marshal(envelope{Type: eventType, Data: data})
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
		}
	}
}

// BroadcastTraffic pushes a captured exchange summary.
func (h *Hub) BroadcastTraffic(s storage.RequestSummary) {
	h.Broadcast("traffic", s)
}

// BroadcastIntercept pushes the current interception state.
func (h *Hub) BroadcastIntercept(state InterceptState) {
	h.Broadcast("intercept", state)
}

func (h *Hub) remove(c *wsClient) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

func (h *Hub) readPump(c *wsClient) {
	defer func() {
		h.remove(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) writePump(c *wsClient) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
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
