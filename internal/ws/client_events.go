package ws

type ClientEvent string

const (
	ClientEventLogin          ClientEvent = "login"
	ClientEventBet            ClientEvent = "bet"
	ClientEventCashout        ClientEvent = "cashout"
	ClientEventSeriesContinue ClientEvent = "series_continue"
)
