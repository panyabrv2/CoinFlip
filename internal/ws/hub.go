package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu sync.RWMutex

	conns  map[*websocket.Conn]struct{}
	authed map[*websocket.Conn]struct{}

	userID map[*websocket.Conn]int64
}

func NewHub() *Hub {
	return &Hub{
		conns:  make(map[*websocket.Conn]struct{}),
		authed: make(map[*websocket.Conn]struct{}),
		userID: make(map[*websocket.Conn]int64),
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
	delete(h.userID, c)
	h.mu.Unlock()
}

func (h *Hub) MarkAuthed(c *websocket.Conn, uid int64) {
	h.mu.Lock()
	if _, ok := h.conns[c]; ok {
		h.authed[c] = struct{}{}
		h.userID[c] = uid
	}
	h.mu.Unlock()
}

func (h *Hub) UserID(c *websocket.Conn) int64 {
	h.mu.RLock()
	uid := h.userID[c]
	h.mu.RUnlock()
	return uid
}

func (h *Hub) Online() int {
	h.mu.RLock()
	n := len(h.authed)
	h.mu.RUnlock()
	return n
}

func (h *Hub) SendJSON(c *websocket.Conn, v any) error {
	if err := c.WriteJSON(v); err != nil {
		log.Printf("hub: send fail ip=%s err=%v", c.RemoteAddr(), err)
		return err
	}
	return nil
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
