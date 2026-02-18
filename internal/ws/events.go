package ws

type Event string

const (
	EventFirstUpdate   Event = "firstUpdate"
	EventAuthorized    Event = "authorized"
	EventOnline        Event = "online"
	EventGameStarted   Event = "gameStarted"
	EventGettingResult Event = "gettingResult"
	EventGameFinished  Event = "gameFinished"
	EventNewGame       Event = "newGame"
	EventBetsAccepted  Event = "bets_accepted"
	EventNewBets       Event = "new_bets"
)
