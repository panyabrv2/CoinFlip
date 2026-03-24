package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type connState struct {
	userID int64
	authed bool
	writeM sync.Mutex
}

type Hub struct {
	mu    sync.RWMutex
	conns map[*websocket.Conn]*connState
}

func NewHub() *Hub {
	return &Hub{
		conns: make(map[*websocket.Conn]*connState),
	}
}

func (h *Hub) Register(c *websocket.Conn) {
	h.mu.Lock()
	h.conns[c] = &connState{}
	h.mu.Unlock()
}

func (h *Hub) Unregister(c *websocket.Conn) {
	h.mu.Lock()
	delete(h.conns, c)
	h.mu.Unlock()
}

func (h *Hub) MarkAuthed(c *websocket.Conn, uid int64) {
	h.mu.Lock()
	if st, ok := h.conns[c]; ok && st != nil {
		st.authed = true
		st.userID = uid
	}
	h.mu.Unlock()
}

func (h *Hub) UserID(c *websocket.Conn) int64 {
	h.mu.RLock()
	st := h.conns[c]
	h.mu.RUnlock()

	if st == nil {
		return 0
	}
	return st.userID
}

func (h *Hub) Online() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	n := 0
	for _, st := range h.conns {
		if st != nil && st.authed {
			n++
		}
	}
	return n
}

func (h *Hub) writeJSON(c *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	h.mu.RLock()
	st := h.conns[c]
	h.mu.RUnlock()

	if st == nil {
		return fmt.Errorf("connection not registered")
	}

	st.writeM.Lock()
	defer st.writeM.Unlock()

	_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.WriteMessage(websocket.TextMessage, b)
}

func (h *Hub) SendJSON(c *websocket.Conn, v any) error {
	if err := h.writeJSON(c, v); err != nil {
		log.Printf("hub: send fail ip=%s err=%v", c.RemoteAddr(), err)
		return err
	}
	return nil
}

func (h *Hub) BroadcastJSON(v any) {
	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.conns))
	for c, st := range h.conns {
		if st != nil && st.authed {
			conns = append(conns, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range conns {
		_ = h.SendJSON(c, v)
	}
}

func (h *Hub) SendToUser(userID int64, v any) {
	h.mu.RLock()
	conns := make([]*websocket.Conn, 0)
	for c, st := range h.conns {
		if st != nil && st.authed && st.userID == userID {
			conns = append(conns, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range conns {
		_ = h.SendJSON(c, v)
	}
}
