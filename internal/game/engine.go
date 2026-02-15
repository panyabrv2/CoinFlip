package game

import (
	"log"
	"sync"

	"CoinFlip/internal/config"
)

type Engine struct {
	mu sync.Mutex

	Phase  string
	Timer  int
	GameID int
	Hash   string

	cfg *config.Config
}

func (e *Engine) Snapshot() (phase string, timer int, gameID int, hash string) {
	e.mu.Lock()
	defer e.mu.Unlock()
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
	}
}

func (e *Engine) NextPhase() {

	switch e.Phase {

	case PhaseWaiting:
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
		log.Println("gameFinished")

	case PhaseFinished:
		e.GameID++
		e.Phase = PhaseWaiting
		e.Timer = 3
		log.Println("newGame")
	}
}
