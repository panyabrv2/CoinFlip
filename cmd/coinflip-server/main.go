package main

import (
	"CoinFlip/internal/config"
	"CoinFlip/internal/game"
	"CoinFlip/internal/ws"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func eventForPhase(e *game.Engine) any {
	switch e.Phase {
	case game.PhaseBetting:
		return ws.GameStarted{
			Event:       "gameStarted",
			GameID:      e.GameID,
			Hash:        e.Hash,
			BettingTime: e.Timer,
		}

	case game.PhaseGettingResult:
		return ws.GettingResult{
			Event:          "gettingResult",
			GameID:         e.GameID,
			Hash:           e.Hash,
			TimeTillResult: e.Timer,
			ResultSide:     "stub",
		}

	case game.PhaseFinished:
		return ws.GameFinished{
			Event:      "gameFinished",
			GameID:     e.GameID,
			Hash:       e.Hash,
			ResultSide: "stub",
		}

	case game.PhaseWaiting:
		return ws.NewGame{
			Event:  "newGame",
			GameID: e.GameID,
			Hash:   e.Hash,
		}
	default:
		return nil
	}
}

func main() {
	cfg := config.Load()
	engine := game.NewEngine(cfg)
	hub := ws.NewHub()

	http.Handle("/ws", &ws.Handler{
		Upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		Engine:   engine,
		Hub:      hub,
	})

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		onlineTick := 0

		for range ticker.C {
			online := hub.Online()
			if online == 0 {
				continue
			}

			oldPhase := engine.Phase

			if engine.Timer > 0 {
				engine.Timer--
			}
			if engine.Timer == 0 {
				engine.NextPhase()
			}

			var evt any
			if engine.Phase != oldPhase {
				evt = eventForPhase(engine)
			}

			onlineTick++
			var onlineEvt any
			if cfg.OnlineInterval > 0 && onlineTick%cfg.OnlineInterval == 0 {
				onlineEvt = ws.OnlineMsg{Event: "online", Online: online}
			}

			if evt != nil {
				hub.BroadcastJSON(evt)
			}
			if onlineEvt != nil {
				hub.BroadcastJSON(onlineEvt)
			}
		}
	}()

	log.Println("SERVER STARTED")
	err := http.ListenAndServe(":8080", nil)
	log.Printf("ListenAndServe returned: %v", err)

}
