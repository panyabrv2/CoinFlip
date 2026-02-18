package ws

type ClientEvent string

const (
	ClientEventLogin ClientEvent = "login"
	ClientEventBet   ClientEvent = "bet"
)
