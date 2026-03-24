package ws

type Event string

const (
	EventFirstUpdate   Event = "firstUpdate"
	EventAuthorized    Event = "authorized"
	EventOnline        Event = "online"
	EventGameStarted   Event = "gameStarted"
	EventGettingResult Event = "gettingResult"
	EventGameFinished  Event = "gameFinished"
	EventCashout       Event = "cashout_result"
	EventNewGame       Event = "newGame"
	EventBetsAccepted  Event = "bets_accepted"
	EventNewBets       Event = "new_bets"
	EventSeriesUpdate  Event = "series_update"
	EventSeriesState   Event = "series_state"
	EventError         Event = "error"
)
