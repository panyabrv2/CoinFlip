package ws

import (
	"CoinFlip/internal/game"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type Handler struct {
	Upgrader websocket.Upgrader
	Engine   *game.Engine
	Hub      *Hub
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("panic in ws handler: %v", rec)
		}
	}()

	conn, err := h.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer func() { _ = conn.Close() }()

	h.Hub.Register(conn)
	defer h.Hub.Unregister(conn)

	log.Println("connected successfully")

	// ---- NEW SNAPSHOT API ----
	snap := h.Engine.Snapshot()
	now := time.Now().UTC().Format(time.RFC3339)

	_ = h.Hub.SendJSON(conn, FirstUpdate{
		Event:      EventFirstUpdate,
		GamePhase:  string(snap.Phase),
		Timer:      snap.Timer,
		GameID:     snap.GameID,
		Hash:       snap.Hash,
		Bets:       h.Engine.BetsSnapshot(),
		ServerTime: now,
	})

	// --- LOGIN ---
	var login LoginMsg
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
	if err := conn.ReadJSON(&login); err != nil {
		log.Printf("login read error: %v", err)
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	if login.ClientEvent != ClientEventLogin || login.Token == "" {
		_ = conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(1008, "login required"),
		)
		return
	}

	h.Hub.MarkAuthed(conn)

	snap = h.Engine.Snapshot()

	_ = h.Hub.SendJSON(conn, Authorized{
		Event:      EventAuthorized,
		GameID:     snap.GameID,
		Hash:       snap.Hash,
		Online:     h.Hub.Online(),
		ServerTime: time.Now().UTC().Format(time.RFC3339),
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var base struct {
			ClientEvent ClientEvent `json:"client_event"`
		}
		if err := json.Unmarshal(raw, &base); err != nil {
			continue
		}

		switch base.ClientEvent {

		case ClientEventBet:
			var bet BetMsg
			if err := json.Unmarshal(raw, &bet); err != nil {
				continue
			}

			if bet.UserID == 0 || len(bet.BetItems) == 0 {
				continue
			}

			items := make([]game.ItemRef, 0, len(bet.BetItems))
			for _, it := range bet.BetItems {
				items = append(items, game.ItemRef{
					Type:   it.Type,
					ItemID: it.ItemID,
				})
			}

			accepted, ok := h.Engine.AddBet(bet.UserID, bet.Side, items)
			if !ok {
				continue
			}

			_ = h.Engine.TryStartFromWaiting()

			snap = h.Engine.Snapshot()

			_ = h.Hub.SendJSON(conn, BetsAccepted{
				Event:      EventBetsAccepted,
				GameID:     snap.GameID,
				Hash:       snap.Hash,
				Accepted:   accepted,
				ServerTime: time.Now().UTC().Format(time.RFC3339),
			})

			h.Hub.BroadcastJSON(NewBets{
				Event:      EventNewBets,
				GameID:     snap.GameID,
				Hash:       snap.Hash,
				Bets:       h.Engine.BetsSnapshot(),
				ServerTime: time.Now().UTC().Format(time.RFC3339),
			})
		}
	}
}
