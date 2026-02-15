package main

import (
	"CoinFlip/internal/config"
	"CoinFlip/internal/game"
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

var onlineCount = 0
var authedConns = make(map[*websocket.Conn]bool)

var gameID = 1
var gameHash = "stub_hash"
var gamePhase = "waiting"
var gameTimer = -1

var cfg *config.Config
var engine *game.Engine

var stateMu sync.Mutex

type FirstUpdate struct {
	Event     string      `json:"event"`
	GamePhase string      `json:"game_phase"`
	Timer     int         `json:"timer"`
	GameID    int         `json:"game_id"`
	Hash      string      `json:"hash"`
	Bets      interface{} `json:"bets"`
}

type LoginMsg struct {
	ClientEvent string `json:"client_event"`
	Token       string `json:"token"`
}

type Authorized struct {
	Event  string `json:"event"`
	GameID int    `json:"game_id"`
	Hash   string `json:"hash"`
	Online int    `json:"online"`
}

type OnlineMsg struct {
	Event  string `json:"event"`
	Online int    `json:"online"`
}

type GameStarted struct {
	Event       string `json:"event"`
	GameID      int    `json:"game_id"`
	Hash        string `json:"hash"`
	BettingTime int    `json:"betting_time"`
}

type GettingResult struct {
	Event          string `json:"event"`
	GameID         int    `json:"game_id"`
	Hash           string `json:"hash"`
	TimeTillResult int    `json:"time_till_result"`
	ResultSide     string `json:"result_side"`
}

type GameFinished struct {
	Event      string `json:"event"`
	GameID     int    `json:"game_id"`
	Hash       string `json:"hash"`
	ResultSide string `json:"result_side"`
}

type NewGame struct {
	Event  string `json:"event"`
	GameID int    `json:"game_id"`
	Hash   string `json:"hash"`
}

//const (
//	PhaseWaiting       = "waiting"
//	PhaseBetting       = "betting"
//	PhaseGettingResult = "gettingResult"
//	PhaseFinished      = "finished"
//)

//func nextPhase() {
//	switch gamePhase {
//
//	case PhaseWaiting:
//		gamePhase = PhaseBetting
//		gameTimer = cfg.BettingTime
//		broadcastJSON(GameStarted{
//			Event:       "gameStarted",
//			GameID:      gameID,
//			Hash:        gameHash,
//			BettingTime: cfg.BettingTime,
//		})
//
//	case PhaseBetting:
//		gamePhase = PhaseGettingResult
//		gameTimer = cfg.TimeTillResult
//		resultSide := "heads"
//		broadcastJSON(GettingResult{
//			Event:          "gettingResult",
//			GameID:         gameID,
//			Hash:           gameHash,
//			TimeTillResult: cfg.TimeTillResult,
//			ResultSide:     resultSide,
//		})
//
//	case PhaseGettingResult:
//		gamePhase = PhaseFinished
//		gameTimer = cfg.NextGameDelay
//		broadcastJSON(GameFinished{
//			Event:      "gameFinished",
//			GameID:     gameID,
//			Hash:       gameHash,
//			ResultSide: "heads",
//		})
//
//	case PhaseFinished:
//		gameID++
//		gameHash = "stub_hash"
//		gamePhase = PhaseWaiting
//		if onlineCount > 0 {
//			gameTimer = 3
//		} else {
//			gameTimer = -1
//		}
//
//		broadcastJSON(NewGame{
//			Event:  "newGame",
//			GameID: gameID,
//			Hash:   gameHash,
//		})
//	}
//}

func broadcastJSON(v any) {
	for c := range authedConns {
		_ = c.WriteJSON(v)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Println(err)
		return
	}

	authorized := false
	_ = authorized

	defer conn.Close()
	defer func() {
		stateMu.Lock()
		if authorized {
			delete(authedConns, conn)
			log.Printf("[DISCONNECT] online(before)=%d", onlineCount)
			onlineCount--
		}
		stateMu.Unlock()
	}()

	log.Println("connected successfully")

	stateMu.Lock()
	phase := engine.Phase
	timer := engine.Timer
	gid := engine.GameID
	hash := engine.Hash
	stateMu.Unlock()

	first := FirstUpdate{
		Event:     "firstUpdate",
		GamePhase: phase,
		Timer:     timer,
		GameID:    gid,
		Hash:      hash,
		Bets:      nil,
	}

	conn.WriteJSON(first)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		log.Println("login info: ", string(data))

		if !authorized {
			var msg LoginMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				_ = conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(1008, "bad json"))
				return
			}

			if msg.ClientEvent != "login" {
				_ = conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(1008, "login required"))
				return
			}

			if msg.Token == "" {
				_ = conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(1008, "auth failed"))
				return
			}
			stateMu.Lock()
			authorized = true
			authedConns[conn] = true
			onlineCount++
			if onlineCount == 1 && engine.Phase == game.PhaseWaiting && engine.Timer == -1 {
				gameTimer = 3
			}
			log.Printf("[LOGIN] online=%d phase=%s timer=%d game=%d", onlineCount, gamePhase, gameTimer, gameID)

			gid := engine.GameID
			hash := engine.Hash
			online := onlineCount

			stateMu.Unlock()

			resp := Authorized{
				Event:  "authorized",
				GameID: gid,
				Hash:   hash,
				Online: online,
			}

			if err := conn.WriteJSON(resp); err != nil {
				log.Println("write authorized error:", err)
				return
			}

			log.Println("message after auth:", string(data))

			continue
		}

		_ = authorized
	}
}

func main() {
	cfg = config.Load()
	engine = game.NewEngine(cfg)

	http.HandleFunc("/ws", wsHandler)
	log.Println("Listening on :8080")

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		prevPhase := ""
		prevTimer := 999999
		prevOnline := -1

		for range ticker.C {
			stateMu.Lock()

			if onlineCount == 0 {
				stateMu.Unlock()
				continue
			}

			if engine.Phase == game.PhaseWaiting && engine.Timer == -1 {
				engine.Timer = 3
			}

			if engine.Timer > 0 {
				engine.Timer--
			}

			if engine.Timer == 0 {
				engine.NextPhase()
			}

			log.Printf("[TICK] online=%d phase=%s timer=%d game=%d", onlineCount, engine.Phase, engine.Timer, engine.GameID)

			if engine.Phase != prevPhase || engine.Timer != prevTimer || onlineCount != prevOnline {
				log.Printf("[STATE] online=%d phase=%s timer=%d game=%d", onlineCount, engine.Phase, engine.Timer, engine.GameID)
				prevPhase = engine.Phase
				prevTimer = engine.Timer
				prevOnline = onlineCount
			}
			stateMu.Unlock()
		}
	}()

	http.ListenAndServe(":8080", nil)
}
