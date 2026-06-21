package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/utkarshrai2811/redtrace/internal/crawler"
	"github.com/utkarshrai2811/redtrace/internal/intruder"
	"github.com/utkarshrai2811/redtrace/internal/oob"
	"github.com/utkarshrai2811/redtrace/internal/scanner"
	"github.com/utkarshrai2811/redtrace/internal/sequencer"
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
	// Accept same-origin and loopback origins (the local UI / dev server), plus
	// non-browser clients that send no Origin. A page on any other site is
	// rejected so it can't open the traffic stream.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		if strings.EqualFold(u.Host, r.Host) {
			return true
		}
		host := u.Hostname()
		if strings.EqualFold(host, "localhost") {
			return true
		}
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	},
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

// BroadcastIntruder pushes an Intruder attack progress update.
func (h *Hub) BroadcastIntruder(update intruder.Update) {
	h.Broadcast("intruder", update)
}

// BroadcastScanner pushes a Scanner finding or active-scan progress update.
func (h *Hub) BroadcastScanner(update scanner.Update) {
	h.Broadcast("scanner", update)
}

// BroadcastCrawl pushes a Crawler discovery or progress update.
func (h *Hub) BroadcastCrawl(update crawler.Update) {
	h.Broadcast("crawl", update)
}

// BroadcastSequencer pushes a Sequencer capture progress update.
func (h *Hub) BroadcastSequencer(update sequencer.Update) {
	h.Broadcast("sequencer", update)
}

// BroadcastOOB pushes a captured out-of-band interaction (without raw bytes).
func (h *Hub) BroadcastOOB(i oob.Interaction) {
	h.Broadcast("oob", oobInteractionView{
		ID: i.ID, Token: i.Token, Protocol: i.Protocol, SourceIP: i.SourceIP,
		Query: i.Query, Detail: i.Detail, CreatedAt: i.CreatedAt,
	})
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
