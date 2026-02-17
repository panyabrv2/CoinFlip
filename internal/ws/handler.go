package ws

import (
	"encoding/json"
	"log"
	"net/http"

	"CoinFlip/internal/game"

	"github.com/gorilla/websocket"
)

type Handler struct {
	Upgrader websocket.Upgrader
	Engine   *game.Engine
	Hub      *Hub
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	conn, err := h.Upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Println(err)
		return
	}

	authorized := false

	defer conn.Close()
	defer func() {
		if authorized {
			h.Hub.RemoveConn(conn)
			log.Printf("[DISCONNECT] online(before)=%d", h.Hub.Online())
		}
	}()

	log.Println("connected successfully")

	first := FirstUpdate{
		Event:     "firstUpdate",
		GamePhase: h.Engine.Phase,
		Timer:     h.Engine.Timer,
		GameID:    h.Engine.GameID,
		Hash:      h.Engine.Hash,
		Bets:      nil,
	}

	_ = conn.WriteJSON(first)

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
			authorized = true
			h.Hub.AddConn(conn)

			resp := Authorized{
				Event:  "authorized",
				GameID: h.Engine.GameID,
				Hash:   h.Engine.Hash,
				Online: h.Hub.Online(),
			}

			if err := conn.WriteJSON(resp); err != nil {
				log.Println("write authorized error:", err)
				return
			}

			log.Println("message after auth:", string(data))

			continue
		}
	}
}
