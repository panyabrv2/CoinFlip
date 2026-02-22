package ws

import (
	"CoinFlip/internal/game"
	"CoinFlip/internal/pricing"
	"CoinFlip/internal/wallet"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Handler struct {
	Upgrader   websocket.Upgrader
	Engine     *game.Engine
	Hub        *Hub
	PriceStore *pricing.Store
	Wallet     *wallet.Store

	TokenStore *TokenStore
}

func (h *Handler) sendErr(conn *websocket.Conn, msg string) {
	_ = h.Hub.SendJSON(conn, ErrorMsg{
		Event:   EventError,
		Message: msg,
	})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip := r.RemoteAddr

	conn, err := h.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade fail ip=%s err=%v", ip, err)
		return
	}
	defer func() {
		uid := h.Hub.UserID(conn)
		log.Printf("ws: disconnect ip=%s uid=%d", ip, uid)
		_ = conn.Close()
	}()

	h.Hub.Register(conn)
	defer h.Hub.Unregister(conn)

	log.Printf("ws: connect ip=%s", ip)

	// firstUpdate before auth
	snap := h.Engine.Snapshot()
	_ = h.Hub.SendJSON(conn, FirstUpdate{
		Event:     EventFirstUpdate,
		GamePhase: string(snap.Phase),
		Timer:     snap.Timer,
		GameID:    snap.GameID,
		Hash:      snap.Hash,
		Bets:      h.Engine.BetsSnapshot(),
	})

	var login LoginMsg
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
	if err := conn.ReadJSON(&login); err != nil {
		log.Printf("ws: auth fail ip=%s reason=read_error err=%v", ip, err)
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	if login.ClientEvent != ClientEventLogin || strings.TrimSpace(login.Token) == "" {
		log.Printf("ws: auth fail ip=%s reason=login_required", ip)
		_ = conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(1008, "login required"),
		)
		return
	}

	if h.TokenStore == nil {
		log.Printf("ws: auth fail ip=%s reason=token_store_nil", ip)
		_ = conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(1011, "server misconfigured: token store"),
		)
		return
	}

	uid, ok := h.TokenStore.Consume(strings.TrimSpace(login.Token))
	if !ok || uid == 0 {
		log.Printf("ws: auth fail ip=%s reason=invalid_token", ip)
		_ = conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(1008, "invalid token"),
		)
		return
	}

	h.Hub.MarkAuthed(conn, uid)
	h.Wallet.EnsureUser(uid)

	log.Printf("ws: auth ok ip=%s uid=%d", ip, uid)

	snap = h.Engine.Snapshot()
	_ = h.Hub.SendJSON(conn, Authorized{
		Event:  EventAuthorized,
		GameID: snap.GameID,
		Hash:   snap.Hash,
		Online: h.Hub.Online(),
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
			log.Printf("ws: reject uid=%d reason=bad_json", h.Hub.UserID(conn))
			h.sendErr(conn, "bad json")
			continue
		}

		switch base.ClientEvent {

		case ClientEventBet:
			var bet BetMsg
			if err := json.Unmarshal(raw, &bet); err != nil {
				log.Printf("ws: reject uid=%d reason=bad_bet_json", h.Hub.UserID(conn))
				h.sendErr(conn, "bad bet json")
				continue
			}

			userID := h.Hub.UserID(conn)
			if userID == 0 {
				log.Printf("ws: reject uid=0 reason=not_authorized")
				h.sendErr(conn, "not authorized")
				continue
			}

			if bet.UserID != 0 && bet.UserID != userID {
				log.Printf("ws: reject uid=%d reason=user_id_mismatch got=%d", userID, bet.UserID)
				h.sendErr(conn, "user_id mismatch")
				continue
			}

			if bet.Side != "heads" && bet.Side != "tails" {
				log.Printf("ws: reject uid=%d reason=bad_side", userID)
				h.sendErr(conn, "bad side")
				continue
			}
			if len(bet.BetItems) == 0 {
				log.Printf("ws: reject uid=%d reason=empty_bet_items", userID)
				h.sendErr(conn, "empty bet_items")
				continue
			}

			items := make([]game.ItemRef, 0, len(bet.BetItems))
			var stakeSum float64

			for _, it := range bet.BetItems {
				price, ok := h.PriceStore.Price(it.Type, it.ItemID)
				if !ok {
					log.Printf("ws: reject uid=%d reason=unknown_price key=%s", userID, pricing.Key(it.Type, it.ItemID))
					h.sendErr(conn, "unknown item price: "+pricing.Key(it.Type, it.ItemID))
					items = nil
					break
				}
				stakeSum += price
				items = append(items, game.ItemRef{
					Type:    it.Type,
					ItemID:  it.ItemID,
					CostTon: price,
				})
			}

			if len(items) == 0 || stakeSum <= 0 {
				continue
			}

			snap = h.Engine.Snapshot()
			gameID := snap.GameID

			if err := h.Wallet.TryReserve(gameID, userID, stakeSum); err != nil {
				log.Printf("ws: reject uid=%d reason=insufficient_funds sum=%.6f", userID, stakeSum)
				h.sendErr(conn, "insufficient funds")
				continue
			}

			accepted, ok := h.Engine.AddBet(userID, bet.Side, items)
			if !ok {
				h.Wallet.Unreserve(gameID, userID, stakeSum)
				log.Printf("ws: reject uid=%d reason=engine_reject", userID)
				h.sendErr(conn, "bet rejected by engine")
				continue
			}

			_ = h.Engine.TryStartFromWaiting()

			snap = h.Engine.Snapshot()

			log.Printf("ws: bet ok uid=%d accepted=%d sum=%.6f", userID, accepted, stakeSum)

			_ = h.Hub.SendJSON(conn, BetsAccepted{
				Event:    EventBetsAccepted,
				GameID:   snap.GameID,
				Hash:     snap.Hash,
				Accepted: accepted,
			})

			h.Hub.BroadcastJSON(NewBets{
				Event:  EventNewBets,
				GameID: snap.GameID,
				Hash:   snap.Hash,
				UserID: userID,
				Side:   bet.Side,
				Bets:   h.Engine.BetsSnapshot(),
			})

		default:
			log.Printf("ws: reject uid=%d reason=unknown_client_event event=%s", h.Hub.UserID(conn), base.ClientEvent)
			h.sendErr(conn, "unknown client_event: "+string(base.ClientEvent))
		}
	}
}
