package ws

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
	Seed       string `json:"seed"` // reveal
}

type NewGame struct {
	Event  string `json:"event"`
	GameID int    `json:"game_id"`
	Hash   string `json:"hash"`
}

type BetMsg struct {
	ClientEvent string    `json:"client_event"` // "bet"
	UserID      int64     `json:"user_id"`
	Side        string    `json:"side"` // "heads" | "tails"
	BetItems    []BetItem `json:"bet_items"`
}

type BetItem struct {
	Type   string `json:"type"`
	ItemID string `json:"item_id"`
}

type BetsAccepted struct {
	Event    string `json:"event"`
	GameID   int    `json:"game_id"`
	Hash     string `json:"hash"`
	Accepted int    `json:"accepted"`
}

type NewBets struct {
	Event  string      `json:"event"`
	GameID int         `json:"game_id"`
	Hash   string      `json:"hash"`
	Bets   interface{} `json:"bets"`
}
