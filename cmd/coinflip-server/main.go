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
				}
			}

			if cfg.OnlineInterval > 0 && onlineTick%cfg.OnlineInterval == 0 {
				hub.BroadcastJSON(ws.OnlineMsg{
					Event:      ws.EventOnline,
					Online:     online,
					ServerTime: t.UTC().Format(time.RFC3339),
				})
			}
		}
	}()

	log.Println("SERVER STARTED")
	err := http.ListenAndServe(":8080", nil)
	log.Printf("ListenAndServe returned: %v", err)
}
