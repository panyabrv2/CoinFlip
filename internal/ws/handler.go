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
		if r := recover(); r != nil {
			log.Printf("panic in ws handler: %v", r)
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

	phase, timer, gameID, hash := h.Engine.Snapshot()
	_ = h.Hub.SendJSON(conn, FirstUpdate{
		Event:     "firstUpdate",
		GamePhase: phase,
		Timer:     timer,
		GameID:    gameID,
		Hash:      hash,
		Bets:      nil,
	})

	var login LoginMsg
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
	if err := conn.ReadJSON(&login); err != nil {
		log.Printf("login read error: %v", err)
		return
	}

	_ = conn.SetReadDeadline(time.Time{})

	if login.ClientEvent != "login" || login.Token == "" {
		_ = conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(1008, "login required"),
		)
		return
	}

	h.Hub.MarkAuthed(conn)

	phase, timer, gameID, hash = h.Engine.Snapshot()
	_, _ = phase, timer
	_ = h.Hub.SendJSON(conn, Authorized{
		Event:  "authorized",
		GameID: gameID,
		Hash:   hash,
		Online: h.Hub.Online(),
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var base struct {
			ClientEvent string `json:"client_event"`
		}
		if err := json.Unmarshal(raw, &base); err != nil {
			continue
		}

		switch base.ClientEvent {
		case "bet":
			var bet BetMsg
			if err := json.Unmarshal(raw, &bet); err != nil {
				continue
			}

			if bet.UserID == 0 || (bet.Side != "heads" && bet.Side != "tails") || len(bet.BetItems) == 0 {
				continue
			}

			items := make([]game.ItemRef, 0, len(bet.BetItems))
			for _, it := range bet.BetItems {
				items = append(items, game.ItemRef{Type: it.Type, ItemID: it.ItemID})
			}

			accepted, ok := h.Engine.AddBet(bet.UserID, bet.Side, items)
			if !ok {
				continue
			}

			_ = h.Engine.TryStartFromWaiting()
			phase, timer, gameID, hash := h.Engine.Snapshot()
			_, _ = phase, timer

			_ = h.Hub.SendJSON(conn, BetsAccepted{
				Event:    "bets_accepted",
				GameID:   gameID,
				Hash:     hash,
				Accepted: accepted,
			})

			h.Hub.BroadcastJSON(NewBets{
				Event:  "new_bets",
				GameID: gameID,
				Hash:   hash,
				Bets:   h.Engine.BetsSnapshot(),
			})
		default:
		}
	}
}
