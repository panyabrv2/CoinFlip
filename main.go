package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

var onlineCount = 0
var authedConns = make(map[*websocket.Conn]bool)

var gameID = 1
var gameHash = "stub_hash"
var gamePhase = "waiting"
var gameTimer = -1

var bettingTimeSec = 10
var timeTillResultSec = 5
var nextGameDelaySec = 2

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

const (
	PhaseWaiting       = "waiting"
	PhaseBetting       = "betting"
	PhaseGettingResult = "gettingResult"
	PhaseFinished      = "finished"
)

func nextPhase() {
	switch gamePhase {

	case PhaseWaiting:
		gamePhase = PhaseBetting
		gameTimer = bettingTimeSec
		broadcastJSON(GameStarted{
			Event:       "gameStarted",
			GameID:      gameID,
			Hash:        gameHash,
			BettingTime: bettingTimeSec,
		})

	case PhaseBetting:
		gamePhase = PhaseGettingResult
		gameTimer = timeTillResultSec
		resultSide := "heads"
		broadcastJSON(GettingResult{
			Event:          "gettingResult",
			GameID:         gameID,
			Hash:           gameHash,
			TimeTillResult: timeTillResultSec,
			ResultSide:     resultSide,
		})

	case PhaseGettingResult:
		gamePhase = PhaseFinished
		gameTimer = nextGameDelaySec
		broadcastJSON(GameFinished{
			Event:      "gameFinished",
			GameID:     gameID,
			Hash:       gameHash,
			ResultSide: "heads",
		})

	case PhaseFinished:
		gameID++
		gameHash = "stub_hash"
		gamePhase = PhaseWaiting
		if onlineCount > 0 {
			gameTimer = 3
		} else {
			gameTimer = -1
		}

		broadcastJSON(NewGame{
			Event:  "newGame",
			GameID: gameID,
			Hash:   gameHash,
		})
	}
}

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
		if authorized {
			delete(authedConns, conn)
			log.Printf("[DISCONNECT] online(before)=%d", onlineCount)
			onlineCount--
		}
	}()

	log.Println("connected successfully")

	first := FirstUpdate{
		Event:     "firstUpdate",
		GamePhase: gamePhase,
		Timer:     gameTimer,
		GameID:    gameID,
		Hash:      gameHash,
		Bets:      nil,
	}

	conn.WriteJSON(first)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		log.Println("raw message: ", string(data))

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

			authorized = true
			authedConns[conn] = true
			onlineCount++
			if onlineCount == 1 && gamePhase == PhaseWaiting && gameTimer == -1 {
				gameTimer = 3
			}
			log.Printf("[LOGIN] online=%d phase=%s timer=%d game=%d", onlineCount, gamePhase, gameTimer, gameID)

			resp := Authorized{
				Event:  "authorized",
				GameID: gameID,
				Hash:   gameHash,
				Online: onlineCount,
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
	gamePhase = PhaseWaiting
	gameTimer = -1
	http.HandleFunc("/ws", wsHandler)
	log.Println("Listening on :8080")

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if onlineCount == 0 {
				continue
			}

			if gameTimer > 0 {
				gameTimer--
			}
			log.Printf("[TICK] online=%d phase=%s timer=%d game=%d", onlineCount, gamePhase, gameTimer, gameID)

			if gameTimer == 0 {
				nextPhase()
			}
		}
	}()

	http.ListenAndServe(":8080", nil)
}
