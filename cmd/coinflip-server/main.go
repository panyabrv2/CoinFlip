package main

import (
	"CoinFlip/internal/config"
	"CoinFlip/internal/game"
	"CoinFlip/internal/pricing"
	"CoinFlip/internal/ws"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	cfg := config.Load()

	priceStore, err := pricing.LoadFromFile(cfg.PricesFile)
	if err != nil {
		log.Fatalf("pricing load error: %v", err)
	}

	engine := game.NewEngine(cfg)
	hub := ws.NewHub()

	http.Handle("/ws", &ws.Handler{
		Upgrader:   websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		Engine:     engine,
		Hub:        hub,
		PriceStore: priceStore,
	})

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		onlineTick := 0

		for t := range ticker.C {
			onlineTick++

			online := hub.Online()
			if online == 0 {
				continue
			}

			phaseChanged, snap := engine.Tick()

			if phaseChanged {
				evt := ws.EventForPhase(snap, t.UTC())
				if evt != nil {
					hub.BroadcastJSON(evt)
					if snap.Phase == game.PhaseFinished {
						if pr, ok := engine.PayoutForGame(snap.GameID); ok {
							hub.BroadcastJSON(ws.GamePayout{
								Event:            ws.EventGamePayout,
								GameID:           pr.GameID,
								Hash:             pr.Hash,
								ResultSide:       string(pr.ResultSide),
								TotalBank:        pr.TotalBank,
								TotalWinning:     pr.TotalWinning,
								HouseCut:         pr.HouseCut,
								Distributate:     pr.Distributable,
								HasWinners:       pr.HasWinners,
								Winners:          pr.Winners,
								HouseProfitTotal: pr.HouseProfitTotal,
								ServerTime:       t.UTC().Format("2006-01-02 15:04:05.000000-07"),
							})
						}
					}
				}
			}

			if cfg.OnlineInterval > 0 && onlineTick%cfg.OnlineInterval == 0 {
				hub.BroadcastJSON(ws.OnlineMsg{
					Event:      ws.EventOnline,
					Online:     online,
					ServerTime: t.UTC().Format("2006-01-02 15:04:05.000000-07"),
				})
			}
		}
	}()

	log.Println("SERVER STARTED")
	err = http.ListenAndServe(":8080", nil)
	log.Printf("ListenAndServe returned: %v", err)
}
