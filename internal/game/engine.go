package game

import (
	"CoinFlip/internal/config"
	"log"
)

type Engine struct {
	Phase      string
	Timer      int
	GameID     int
	Hash       string
	ResultSide string
	bets       *BetStore

	cfg *config.Config
}

func (e *Engine) Snapshot() (phase string, timer int, gameID int, hash string) {
	return e.Phase, e.Timer, e.GameID, e.Hash
}

const (
	PhaseWaiting       = "waiting"
	PhaseBetting       = "betting"
	PhaseGettingResult = "gettingResult"
	PhaseFinished      = "finished"
)

func NewEngine(cfg *config.Config) *Engine {
	return &Engine{
		Phase:  PhaseWaiting,
		Timer:  -1,
		GameID: 1,
		Hash:   "stub_hash",
		cfg:    cfg,
		bets:   NewBetStore(),
	}
}

func (e *Engine) NextPhase() {
	switch e.Phase {

	case PhaseWaiting:
		if e.UniqueBetUsers() < 2 {
			e.Timer = -1
			return
		}
		e.Phase = PhaseBetting
		e.Timer = e.cfg.BettingTime
		log.Println("gameStarted")

	case PhaseBetting:
		e.Phase = PhaseGettingResult
		e.Timer = e.cfg.TimeTillResult
		log.Println("gettingResult")

	case PhaseGettingResult:
		e.Phase = PhaseFinished
		e.Timer = e.cfg.NextGameDelay
		e.ResultSide = "heads"
		log.Println("gameFinished")

	case PhaseFinished:
		e.bets.Reset(e.GameID)

		e.GameID++
		e.Phase = PhaseWaiting
		e.Timer = -1
		log.Println("newGame")
	}
}

func (e *Engine) AddBet(userID int64, side string, items []ItemRef) (accepted int, ok bool) {
	if userID == 0 {
		return 0, false
	}
	if side != "heads" && side != "tails" {
		return 0, false
	}
	if len(items) == 0 {
		return 0, false
	}

	accepted = e.bets.Add(e.GameID, userID, side, items)
	return accepted, true
}

func (e *Engine) UniqueBetUsers() int {
	return e.bets.UniqueUsers(e.GameID)
}

func (e *Engine) BetsSnapshot() any {
	return e.bets.Snapshot(e.GameID)
}

func (e *Engine) TryStartFromWaiting() bool {
	if e.Phase != PhaseWaiting {
		return false
	}
	if e.UniqueBetUsers() < 2 {
		return false
	}
	e.Phase = PhaseBetting
	e.Timer = e.cfg.BettingTime
	log.Println("gameStarted")
	return true
}
