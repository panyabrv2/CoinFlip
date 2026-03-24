package main

import (
	"CoinFlip/internal/config"
	"CoinFlip/internal/game"
	"CoinFlip/internal/storage/postgres"
	"CoinFlip/internal/ws"
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	dbPool, err := postgres.NewPool(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("postgres connect err=%v", err)
	}
	defer dbPool.Close()
	log.Println("postgres: connected")

	itemsRepo := postgres.NewItemsRepo(dbPool)
	usersRepo := postgres.NewUsersRepo(dbPool)
	gamesRepo := postgres.NewGamesRepo(dbPool)
	betsRepo := postgres.NewBetsRepo(dbPool)
	seriesRepo := postgres.NewSeriesRepo(dbPool)

	nextGameID, err := gamesRepo.NextGameID(ctx)
	if err != nil {
		log.Fatalf("games next game id err=%v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis connect err=%v", err)
	}
	defer func() { _ = rdb.Close() }()
	log.Println("redis: connected")

	engine := game.NewEngine(cfg, nextGameID)
	hub := ws.NewHub()
	tokens := ws.NewTokenStore(rdb)

	h := &ws.Handler{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		Engine:             engine,
		Hub:                hub,
		TokenStore:         tokens,
		TokenTouchInterval: time.Duration(cfg.RedisTokenTouchSeconds) * time.Second,
		ItemsRepo:          itemsRepo,
		UsersRepo:          usersRepo,
		BetsRepo:           betsRepo,
		SeriesRepo:         seriesRepo,
	}

	initialSnap := engine.Snapshot()
	if err := gamesRepo.EnsureRound(ctx, initialSnap.GameID, string(initialSnap.Phase), initialSnap.Hash, initialSnap.Seed); err != nil {
		log.Fatalf("games ensure initial round err=%v", err)
	}

	http.Handle("/ws", h)

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		onlineTick := 0

		for range ticker.C {
			onlineTick++

			if onlineTick%30 == 0 && tokens != nil {
				n, err := tokens.CleanupStale(ctx, cfg.RedisTokenStaleSeconds)
				if err != nil {
					log.Printf("redis: cleanup stale err=%v", err)
				} else if n > 0 {
					log.Printf("redis: cleanup stale unlocked=%d", n)
				}
			}

			online := hub.Online()
			snapBefore := engine.Snapshot()

			if online == 0 && snapBefore.Phase == game.PhaseWaiting {
				continue
			}

			phaseChanged, snap := engine.Tick(online > 0)

			if phaseChanged {
				switch snap.Phase {
				case game.PhaseWaiting, game.PhaseBetting, game.PhaseGettingResult:
					if err := gamesRepo.EnsureRound(ctx, snap.GameID, string(snap.Phase), snap.Hash, snap.Seed); err != nil {
						log.Printf("games: ensure round err=%v", err)
					}
					if err := gamesRepo.SetPhase(ctx, snap.GameID, string(snap.Phase)); err != nil {
						log.Printf("games: set phase err=%v", err)
					}

				case game.PhaseFinished:
					if err := gamesRepo.FinishRound(ctx, snap.GameID, string(snap.ResultSide), snap.Seed); err != nil {
						log.Printf("games: finish round err=%v", err)
					}

					if sr, ok := engine.SeriesResultsForGame(snap.GameID); ok {
						for uid, res := range sr {
							switch res.Outcome {
							case "win":
								if _, err := seriesRepo.MoveToAwaitingChoiceAfterWin(
									ctx,
									uid,
									snap.GameID,
									res.Side,
									res.Wins,
									res.Multiplier,
									res.Claimable,
								); err != nil {
									log.Printf("series: move awaiting_choice err user=%d err=%v", uid, err)
								}

							case "lose":
								if _, err := seriesRepo.MarkLost(
									ctx,
									uid,
									snap.GameID,
									res.Side,
									res.Wins,
									res.Multiplier,
								); err != nil {
									log.Printf("series: mark lost err user=%d err=%v", uid, err)
								}
							}
						}
					}

					itemIDs, err := betsRepo.ItemIDsForGame(ctx, snap.GameID)
					if err != nil {
						log.Printf("bets: item ids for game err=%v", err)
					} else if len(itemIDs) > 0 {
						n, err := itemsRepo.ConsumeLockedItems(ctx, itemIDs)
						if err != nil {
							log.Printf("items: consume locked items err=%v", err)
						} else {
							log.Printf("items: consumed locked items game_id=%d n=%d", snap.GameID, n)
						}
					}

				}

				if evt := ws.EventForPhase(snap); evt != nil {
					hub.BroadcastJSON(evt)
				}

				if snap.Phase == game.PhaseFinished {
					if sr, ok := engine.SeriesResultsForGame(snap.GameID); ok {
						for uid, res := range sr {
							hub.SendToUser(uid, ws.SeriesUpdate{
								Event:      ws.EventSeriesUpdate,
								GameID:     snap.GameID,
								UserID:     uid,
								Side:       res.Side,
								Stake:      res.Stake,
								Wins:       res.Wins,
								Multiplier: res.Multiplier,
								Claimable:  res.Claimable,
								Stage:      string(res.Stage),
								Active:     res.Active,
								Outcome:    res.Outcome,
							})
						}
					}
				}
			}

			if online > 0 && cfg.OnlineInterval > 0 && onlineTick%cfg.OnlineInterval == 0 {
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
