package ws

type FirstUpdate struct {
	Event     Event       `json:"event"`
	GamePhase string      `json:"game_phase"`
	Timer     int         `json:"timer"`
	GameID    int         `json:"game_id"`
	Hash      string      `json:"hash"`
	Bets      interface{} `json:"bets"`
}

type LoginMsg struct {
	ClientEvent ClientEvent `json:"client_event"`
	Token       string      `json:"token"`
}

type Authorized struct {
	Event  Event  `json:"event"`
	GameID int    `json:"game_id"`
	Hash   string `json:"hash"`
	Online int    `json:"online"`
}

type OnlineMsg struct {
	Event  Event `json:"event"`
	Online int   `json:"online"`
}

type GameStarted struct {
	Event       Event  `json:"event"`
	GameID      int    `json:"game_id"`
	Hash        string `json:"hash"`
	BettingTime int    `json:"betting_time"`
}

type GettingResult struct {
	Event          Event  `json:"event"`
	GameID         int    `json:"game_id"`
	Hash           string `json:"hash"`
	TimeTillResult int    `json:"time_till_result"`
	ResultSide     string `json:"result_side"`
}

type GameFinished struct {
	Event      Event  `json:"event"`
	GameID     int    `json:"game_id"`
	Hash       string `json:"hash"`
	ResultSide string `json:"result_side"`
}

type NewGame struct {
	Event  Event  `json:"event"`
	GameID int    `json:"game_id"`
	Hash   string `json:"hash"`
}

type BetMsg struct {
	ClientEvent ClientEvent `json:"client_event"`
	UserID      int64       `json:"user_id"`
	Side        string      `json:"side"`
	BetItems    []BetItem   `json:"bet_items"`
}

type BetItem struct {
	Type   string `json:"type"`
	ItemID string `json:"item_id"`
}

type BetsAccepted struct {
	Event    Event  `json:"event"`
	GameID   int    `json:"game_id"`
	Hash     string `json:"hash"`
	Accepted int    `json:"accepted"`
}

type NewBets struct {
	Event  Event       `json:"event"`
	GameID int         `json:"game_id"`
	Hash   string      `json:"hash"`
	UserID int64       `json:"user_id"`
	Side   string      `json:"side"`
	Bets   interface{} `json:"bets"`
}

type ErrorMsg struct {
	Event   Event  `json:"event"`
	Message string `json:"error"`
}
