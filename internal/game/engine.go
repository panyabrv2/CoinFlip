package game

import (
	"CoinFlip/internal/config"
	"CoinFlip/internal/rng"
	"encoding/hex"
	"log"
)

type Snapshot struct {
	Phase      Phase
	Timer      int
	GameID     int
	Hash       string
	ResultSide Side
	Seed       string // hex, reveal
}

type Engine struct {
	phase      Phase
	timer      int
	gameID     int
	hash       string
	resultSide Side
	seedHex    string

	bets *BetStore
	cfg  *config.Config
}

func NewEngine(cfg *config.Config) *Engine {
	seedBytes, err := rng.NewSeed()
	if err != nil {
		log.Printf("rng.NewSeed error: %v", err)
		seedBytes = []byte("fallback-seed-fallback-seed-123456")
	}

	seedHex := hex.EncodeToString(seedBytes)
	hash := rng.SHA1Hex(seedBytes)

	return &Engine{
		phase:   PhaseWaiting,
		timer:   -1,
		gameID:  1,
		seedHex: seedHex,
		hash:    hash,

		bets: NewBetStore(),
		cfg:  cfg,
	}
}

func (e *Engine) Snapshot() Snapshot {
	return Snapshot{
		Phase:      e.phase,
		Timer:      e.timer,
		GameID:     e.gameID,
		Hash:       e.hash,
		ResultSide: e.resultSide,
		Seed:       e.seedHex,
	}
}

// Tick вызывается раз в секунду.
// Возвращает (phaseChanged, snapshotAfterTick).
func (e *Engine) Tick() (bool, Snapshot) {
	old := e.phase

	if e.timer > 0 {
		e.timer--
	}
	if e.timer == 0 {
		e.nextPhase()
	}

	return e.phase != old, e.Snapshot()
}

func (e *Engine) nextPhase() {
	switch e.phase {

	case PhaseWaiting:
		if e.UniqueBetUsers() < 2 {
			e.timer = -1
			return
		}
		e.phase = PhaseBetting
		e.timer = e.cfg.BettingTime
		log.Println("gameStarted")

	case PhaseBetting:
		e.phase = PhaseGettingResult
		e.timer = e.cfg.TimeTillResult

		// commit -> reveal: результат детерминирован из seed
		seedBytes, err := hex.DecodeString(e.seedHex)
		if err != nil {
			e.resultSide = SideHeads
		} else {
			e.resultSide = Side(rng.SideFromSeed(seedBytes))
		}

		log.Println("gettingResult")

	case PhaseGettingResult:
		e.phase = PhaseFinished
		e.timer = e.cfg.NextGameDelay
		log.Println("gameFinished")

	case PhaseFinished:
		e.bets.Reset(e.gameID)

		e.gameID++
		e.phase = PhaseWaiting
		e.timer = -1
		e.resultSide = Side("")

		seedBytes, err := rng.NewSeed()
		if err != nil {
			log.Printf("rng.NewSeed error: %v", err)
			seedBytes = []byte("fallback-seed-fallback-seed-123456")
		}

		e.seedHex = hex.EncodeToString(seedBytes)
		e.hash = rng.SHA1Hex(seedBytes)

		log.Println("newGame")
	}
}

func (e *Engine) AddBet(userID int64, side string, items []ItemRef) (accepted int, ok bool) {
	if userID == 0 {
		return 0, false
	}
	if side != string(SideHeads) && side != string(SideTails) {
		return 0, false
	}
	if len(items) == 0 {
		return 0, false
	}

	switch e.phase {
	case PhaseWaiting:
	case PhaseBetting:
		if e.timer <= 0 {
			return 0, false
		}
	default:
		return 0, false
	}

	accepted = e.bets.Add(e.gameID, userID, side, items)
	return accepted, true
}

func (e *Engine) UniqueBetUsers() int {
	return e.bets.UniqueUsers(e.gameID)
}

func (e *Engine) BetsSnapshot() any {
	return e.bets.Snapshot(e.gameID)
}

func (e *Engine) TryStartFromWaiting() bool {
	if e.phase != PhaseWaiting {
		return false
	}
	if e.UniqueBetUsers() < 2 {
		return false
	}
	e.phase = PhaseBetting
	e.timer = e.cfg.BettingTime
	log.Println("gameStarted")
	return true
}
