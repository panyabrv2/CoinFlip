package ws

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu sync.RWMutex

	conns map[*websocket.Conn]struct{}

	authed map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{
		conns:  make(map[*websocket.Conn]struct{}),
		authed: make(map[*websocket.Conn]struct{}),
	}
}

func (h *Hub) Register(c *websocket.Conn) {
	h.mu.Lock()
	h.conns[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) Unregister(c *websocket.Conn) {
	h.mu.Lock()
	delete(h.conns, c)
	delete(h.authed, c)
	h.mu.Unlock()
}

func (h *Hub) MarkAuthed(c *websocket.Conn) {
	h.mu.Lock()
	if _, ok := h.conns[c]; ok {
		h.authed[c] = struct{}{}
	}
	h.mu.Unlock()
}

func (h *Hub) Online() int {
	h.mu.RLock()
	n := len(h.authed)
	h.mu.RUnlock()
	return n
}

func (h *Hub) SendJSON(c *websocket.Conn, v any) error {
	return c.WriteJSON(v)
}

func (h *Hub) BroadcastJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}

	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.authed))
	for c := range h.authed {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	for _, c := range conns {
		_ = c.WriteMessage(websocket.TextMessage, b)
	}
}
