package main

import (
	"CoinFlip/internal/config"
	"CoinFlip/internal/game"
	"CoinFlip/internal/pricing"
	"CoinFlip/internal/wallet"
	"CoinFlip/internal/ws"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	cfg := config.Load()
	engine := game.NewEngine(cfg)
	hub := ws.NewHub()
	w := wallet.NewStore(100.0)

	priceStore, err := pricing.LoadFromFile(cfg.PricesFile)
	if err != nil {
		log.Fatalf("server: pricing load err=%v", err)
	}

	tokens := ws.NewTokenStore(1)

	http.Handle("/ws", &ws.Handler{
		Upgrader:   websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		Engine:     engine,
		Hub:        hub,
		PriceStore: priceStore,
		Wallet:     w,
		TokenStore: tokens,
	})

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		onlineTick := 0

		for range ticker.C {
			onlineTick++

			online := hub.Online()
			if online == 0 {
				continue
			}

			phaseChanged, snap := engine.Tick()

			if phaseChanged {
				if evt := ws.EventForPhase(snap); evt != nil {
					hub.BroadcastJSON(evt)
				}

				if snap.Phase == game.PhaseFinished {
					if pr, ok := engine.PayoutForGame(snap.GameID); ok {
						winners := make(map[int64]float64, len(pr.Winners))
						for uid, wp := range pr.Winners {
							winners[uid] = wp.Payout
						}
						w.SettleGame(pr.GameID, winners, 0)
						log.Printf("game: settle game_id=%d winners=%d", pr.GameID, len(pr.Winners))
					}
				}
			}

			if cfg.OnlineInterval > 0 && onlineTick%cfg.OnlineInterval == 0 {
				hub.BroadcastJSON(ws.OnlineMsg{
					Event:  ws.EventOnline,
					Online: online,
				})
			}
		}
	}()

	log.Println("server: start addr=:8080")
	err = http.ListenAndServe(":8080", nil)
	log.Printf("server: stop err=%v", err)
}
