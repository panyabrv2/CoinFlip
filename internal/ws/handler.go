package ws

import (
	"CoinFlip/internal/game"
	"CoinFlip/internal/storage/postgres"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Handler struct {
	Upgrader websocket.Upgrader

	Engine             *game.Engine
	Hub                *Hub
	TokenStore         *TokenStore
	TokenTouchInterval time.Duration

	ItemsRepo  *postgres.ItemsRepo
	UsersRepo  *postgres.UsersRepo
	BetsRepo   *postgres.BetsRepo
	SeriesRepo *postgres.SeriesRepo

	muLocked sync.Mutex
	locked   map[int][]int
}

func (h *Handler) ensureLockedMap() {
	h.muLocked.Lock()
	defer h.muLocked.Unlock()

	if h.locked == nil {
		h.locked = make(map[int][]int)
	}
}

func (h *Handler) addLocked(gameID int, ids []int) {
	if len(ids) == 0 {
		return
	}
	h.ensureLockedMap()

	h.muLocked.Lock()
	h.locked[gameID] = append(h.locked[gameID], ids...)
	h.muLocked.Unlock()
}

func (h *Handler) takeLocked(gameID int) []int {
	h.ensureLockedMap()

	h.muLocked.Lock()
	ids := h.locked[gameID]
	delete(h.locked, gameID)
	h.muLocked.Unlock()

	return ids
}

func (h *Handler) UnlockForGame(gameID int) {
	if h.ItemsRepo == nil {
		return
	}

	ids := h.takeLocked(gameID)
	if len(ids) == 0 {
		return
	}

	if err := h.ItemsRepo.UnlockItems(context.Background(), ids); err != nil {
		log.Printf("ws: unlock items fail game_id=%d err=%v", gameID, err)
	} else {
		log.Printf("ws: unlocked items game_id=%d n=%d", gameID, len(ids))
	}
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

	var lockedToken string
	var sessionID string
	stopPing := make(chan struct{})

	defer func() {
		close(stopPing)

		uid := h.Hub.UserID(conn)
		log.Printf("ws: disconnect ip=%s uid=%d", ip, uid)

		if lockedToken != "" && sessionID != "" && h.TokenStore != nil {
			if err := h.TokenStore.UnlockWithSession(context.Background(), lockedToken, sessionID); err != nil {
				log.Printf("ws: token unlock fail ip=%s uid=%d err=%v", ip, uid, err)
			}
		}

		_ = conn.Close()
	}()

	h.Hub.Register(conn)
	defer h.Hub.Unregister(conn)

	log.Printf("ws: connect ip=%s", ip)

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

	if login.ClientEvent != ClientEventLogin || strings.TrimSpace(login.Token) == "" {
		log.Printf("ws: auth fail ip=%s reason=login_required", ip)
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1008, "login required"))
		return
	}

	if h.TokenStore == nil {
		log.Printf("ws: auth fail ip=%s reason=token_store_nil", ip)
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1011, "server misconfigured: token store"))
		return
	}

	token := strings.TrimSpace(login.Token)
	uid, sid, ok, err := h.TokenStore.LockWithSession(context.Background(), token)
	if err != nil || !ok || uid == 0 {
		log.Printf("ws: auth fail ip=%s reason=redis_token err=%v ok=%v uid=%d", ip, err, ok, uid)
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1008, "invalid token"))
		return
	}

	lockedToken = token
	sessionID = sid
	h.Hub.MarkAuthed(conn, uid)

	if h.UsersRepo != nil {
		if err := h.UsersRepo.EnsureUser(context.Background(), uid); err != nil {
			log.Printf("ws: ensure user fail uid=%d err=%v", uid, err)
			h.sendErr(conn, "db error: ensure user")
			return
		}
	}

	touchEvery := h.TokenTouchInterval
	if touchEvery <= 0 {
		touchEvery = 30 * time.Second
	}
	pongWait := touchEvery*2 + 15*time.Second

	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))

		if lockedToken != "" && sessionID != "" && h.TokenStore != nil {
			_ = h.TokenStore.Touch(context.Background(), lockedToken, sessionID)
		}
		return nil
	})

	go func() {
		ticker := time.NewTicker(touchEvery)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if lockedToken != "" && sessionID != "" && h.TokenStore != nil {
					_ = h.TokenStore.Touch(context.Background(), lockedToken, sessionID)
				}
				_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second))

			case <-stopPing:
				return
			}
		}
	}()

	log.Printf("ws: auth ok ip=%s uid=%d", ip, uid)

	snap = h.Engine.Snapshot()
	_ = h.Hub.SendJSON(conn, Authorized{
		Event:  EventAuthorized,
		GameID: snap.GameID,
		Hash:   snap.Hash,
		Online: h.Hub.Online(),
	})

	if ss, ok := h.Engine.SeriesSnapshot(uid); ok {
		_ = h.Hub.SendJSON(conn, SeriesStateMsg{
			Event:      EventSeriesState,
			UserID:     ss.UserID,
			Side:       ss.Side,
			Stake:      ss.Stake,
			Wins:       ss.Wins,
			Multiplier: ss.Multiplier,
			Claimable:  ss.Claimable,
			Stage:      string(ss.Stage),
			Active:     ss.Active,
		})
	}

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}

		if lockedToken != "" && sessionID != "" && h.TokenStore != nil {
			_ = h.TokenStore.Touch(context.Background(), lockedToken, sessionID)
		}

		var base struct {
			ClientEvent ClientEvent `json:"client_event"`
		}
		if err := json.Unmarshal(raw, &base); err != nil {
			h.sendErr(conn, "bad json")
			continue
		}

		switch base.ClientEvent {
		case ClientEventCashout:
			userID := h.Hub.UserID(conn)
			if userID == 0 {
				h.sendErr(conn, "not authorized")
				continue
			}

			prevSS, _ := h.Engine.SeriesSnapshot(userID)

			stake, mult, payout, ok, reason := h.Engine.Cashout(userID)
			if !ok {
				h.sendErr(conn, reason)
				continue
			}

			if h.SeriesRepo != nil {
				snap = h.Engine.Snapshot()
				if _, err := h.SeriesRepo.Cashout(context.Background(), userID, snap.GameID, payout); err != nil {
					if prevSS != nil {
						h.Engine.RestoreSeriesSnapshot(*prevSS)
					}
					h.sendErr(conn, "db error: cashout series")
					continue
				}
			}

			snap = h.Engine.Snapshot()
			_ = h.Hub.SendJSON(conn, CashoutResult{
				Event:      EventCashout,
				GameID:     snap.GameID,
				UserID:     userID,
				Stake:      stake,
				Multiplier: mult,
				Payout:     payout,
			})

			_ = h.Hub.SendJSON(conn, SeriesStateMsg{
				Event:      EventSeriesState,
				UserID:     userID,
				Side:       "",
				Stake:      0,
				Wins:       0,
				Multiplier: 0,
				Claimable:  0,
				Stage:      "",
				Active:     false,
			})

		case ClientEventSeriesContinue:
			var msg SeriesContinueMsg
			if err := json.Unmarshal(raw, &msg); err != nil {
				h.sendErr(conn, "bad series_continue json")
				continue
			}

			userID := h.Hub.UserID(conn)
			if userID == 0 {
				h.sendErr(conn, "not authorized")
				continue
			}

			prevSS, _ := h.Engine.SeriesSnapshot(userID)

			ss, ok, reason := h.Engine.SeriesContinue(userID, msg.Side)
			if !ok {
				h.sendErr(conn, reason)
				continue
			}

			if h.SeriesRepo != nil {
				snap = h.Engine.Snapshot()
				if err := h.SeriesRepo.Continue(context.Background(), userID, snap.GameID, msg.Side); err != nil {
					if prevSS != nil {
						h.Engine.RestoreSeriesSnapshot(*prevSS)
					}
					h.sendErr(conn, "db error: continue series")
					continue
				}
			}

			_ = h.Hub.SendJSON(conn, SeriesStateMsg{
				Event:      EventSeriesState,
				UserID:     ss.UserID,
				Side:       ss.Side,
				Stake:      ss.Stake,
				Wins:       ss.Wins,
				Multiplier: ss.Multiplier,
				Claimable:  ss.Claimable,
				Stage:      string(ss.Stage),
				Active:     ss.Active,
			})

		case ClientEventBet:
			var bet BetMsg
			if err := json.Unmarshal(raw, &bet); err != nil {
				h.sendErr(conn, "bad bet json")
				continue
			}

			userID := h.Hub.UserID(conn)
			if userID == 0 {
				h.sendErr(conn, "not authorized")
				continue
			}

			if bet.UserID != 0 && bet.UserID != userID {
				h.sendErr(conn, "user_id mismatch")
				continue
			}

			if bet.Side != "heads" && bet.Side != "tails" {
				h.sendErr(conn, "bad side")
				continue
			}

			mode := "series"

			if len(bet.BetItems) == 0 {
				h.sendErr(conn, "empty bet_items")
				continue
			}

			if h.ItemsRepo == nil {
				h.sendErr(conn, "server misconfigured: items repo")
				continue
			}
			if h.BetsRepo == nil {
				h.sendErr(conn, "server misconfigured: bets repo")
				continue
			}
			if h.SeriesRepo == nil {
				h.sendErr(conn, "server misconfigured: series repo")
				continue
			}

			ctx := context.Background()

			itemIDs := make([]int, 0, len(bet.BetItems))
			for _, bi := range bet.BetItems {
				id, err := strconv.Atoi(strings.TrimSpace(bi.ItemID))
				if err != nil || id <= 0 {
					h.sendErr(conn, "bad item_id: "+bi.ItemID)
					itemIDs = nil
					break
				}
				itemIDs = append(itemIDs, id)
			}
			if len(itemIDs) == 0 {
				continue
			}

			dbItems, err := h.ItemsRepo.LockItems(ctx, itemIDs, userID)
			if err != nil {
				h.sendErr(conn, "item not found / not owned / already locked")
				continue
			}

			items := make([]game.ItemRef, 0, len(dbItems))
			lockedIDs := make([]int, 0, len(dbItems))

			for _, it := range dbItems {
				lockedIDs = append(lockedIDs, it.ItemID)

				items = append(items, game.ItemRef{
					Type:     it.Type,
					ItemID:   strconv.Itoa(it.ItemID),
					Name:     it.Name,
					PhotoURL: it.PhotoURL,
					CostTon:  it.CostTon,
				})
			}

			snap, accepted, ok, reason := h.Engine.AddBet(userID, bet.Side, mode, items)
			if !ok {
				_ = h.ItemsRepo.UnlockItems(ctx, lockedIDs)
				h.sendErr(conn, reason)
				continue
			}

			totalStake := 0.0
			for _, it := range items {
				totalStake += it.CostTon
			}

			sid, err := h.SeriesRepo.CreateSession(ctx, postgres.CreateSeriesSessionParams{
				UserID:        userID,
				InitialGameID: snap.GameID,
				CurrentSide:   bet.Side,
				StakeTon:      totalStake,
			})
			if err != nil {
				h.Engine.RollbackAcceptedBet(snap.GameID, userID, mode, len(items))
				_ = h.ItemsRepo.UnlockItems(ctx, lockedIDs)
				h.sendErr(conn, "db error: create series session")
				continue
			}
			seriesSessionID := &sid

			rows := make([]postgres.CreateBetRow, 0, len(dbItems))
			for _, it := range dbItems {
				rows = append(rows, postgres.CreateBetRow{
					GameID:          snap.GameID,
					UserID:          userID,
					Side:            bet.Side,
					Mode:            mode,
					SeriesSessionID: seriesSessionID,
					ItemID:          it.ItemID,
					ItemType:        it.Type,
					ItemName:        it.Name,
					ItemPhotoURL:    it.PhotoURL,
					StakeTon:        it.CostTon,
				})
			}

			if err := h.BetsRepo.InsertAcceptedBets(ctx, rows); err != nil {
				_ = h.SeriesRepo.DeleteSession(ctx, *seriesSessionID)
				h.Engine.RollbackAcceptedBet(snap.GameID, userID, mode, len(items))
				_ = h.ItemsRepo.UnlockItems(ctx, lockedIDs)
				h.sendErr(conn, "db error: save bets")
				continue
			}

			h.addLocked(snap.GameID, lockedIDs)

			_ = h.Hub.SendJSON(conn, BetsAccepted{
				Event:    EventBetsAccepted,
				GameID:   snap.GameID,
				Hash:     snap.Hash,
				Accepted: accepted,
			})

			if ss, ok := h.Engine.SeriesSnapshot(userID); ok {
				_ = h.Hub.SendJSON(conn, SeriesStateMsg{
					Event:      EventSeriesState,
					UserID:     ss.UserID,
					Side:       ss.Side,
					Stake:      ss.Stake,
					Wins:       ss.Wins,
					Multiplier: ss.Multiplier,
					Claimable:  ss.Claimable,
					Stage:      string(ss.Stage),
					Active:     ss.Active,
				})
			}

			h.Hub.BroadcastJSON(NewBets{
				Event:  EventNewBets,
				GameID: snap.GameID,
				Hash:   snap.Hash,
				UserID: userID,
				Side:   bet.Side,
				Mode:   "series",
				Bets:   h.Engine.BetsSnapshotForGame(snap.GameID),
			})

		default:
			h.sendErr(conn, "unknown client_event: "+string(base.ClientEvent))
		}
	}
}
