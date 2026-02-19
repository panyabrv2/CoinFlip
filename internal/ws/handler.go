package ws

import (
	"CoinFlip/internal/game"
	"CoinFlip/internal/pricing"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type Handler struct {
	Upgrader   websocket.Upgrader
	Engine     *game.Engine
	Hub        *Hub
	PriceStore *pricing.Store
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

	snap := h.Engine.Snapshot()
	now := time.Now().UTC().Format("2006-01-02 15:04:05.000000-07")

	_ = h.Hub.SendJSON(conn, FirstUpdate{
		Event:      EventFirstUpdate,
		GamePhase:  string(snap.Phase),
		Timer:      snap.Timer,
		GameID:     snap.GameID,
		Hash:       snap.Hash,
		Bets:       h.Engine.BetsSnapshot(),
		History:    h.Engine.History(),
		ServerTime: now,
	})

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
		ServerTime: time.Now().UTC().Format("2006-01-02 15:04:05.000000-07"),
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

			if bet.UserID == 0 ||
				(bet.Side != "heads" && bet.Side != "tails") ||
				len(bet.BetItems) == 0 {
				continue
			}

			items := make([]game.ItemRef, 0, len(bet.BetItems))

			for _, it := range bet.BetItems {
				price, ok := h.PriceStore.Price(it.Type, it.ItemID)
				if !ok {
					_ = h.Hub.SendJSON(conn, ErrorMsg{
						Event:      EventError,
						Message:    "unknown item price: " + pricing.Key(it.Type, it.ItemID),
						ServerTime: time.Now().UTC().Format("2006-01-02 15:04:05.000000-07"),
					})
					items = nil
					break
				}

				items = append(items, game.ItemRef{
					Type:    it.Type,
					ItemID:  it.ItemID,
					CostTon: price,
				})
			}

			if len(items) == 0 {
				continue
			}

			accepted, ok := h.Engine.AddBet(bet.UserID, bet.Side, items)
			if !ok {
				continue
			}

			_ = h.Engine.TryStartFromWaiting()

			snap = h.Engine.Snapshot()
			serverTime := time.Now().UTC().Format("2006-01-02 15:04:05.000000-07")

			_ = h.Hub.SendJSON(conn, BetsAccepted{
				Event:      EventBetsAccepted,
				GameID:     snap.GameID,
				Hash:       snap.Hash,
				Accepted:   accepted,
				ServerTime: serverTime,
			})

			h.Hub.BroadcastJSON(NewBets{
				Event:      EventNewBets,
				GameID:     snap.GameID,
				Hash:       snap.Hash,
				UserID:     bet.UserID,
				Side:       bet.Side,
				Bets:       h.Engine.BetsSnapshot(),
				ServerTime: serverTime,
			})
		}
	}
}
