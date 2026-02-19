package game

import (
	"CoinFlip/internal/config"
	"CoinFlip/internal/rng"
	"encoding/hex"
	"log"
	"sync"
)

type Snapshot struct {
	Phase      Phase
	Timer      int
	GameID     int
	Hash       string
	ResultSide Side
	Seed       string
}

type WinnerPayout struct {
	UserID     int64   `json:"user_id"`
	Stake      float64 `json:"stake"`
	Payout     float64 `json:"payout"`
	Multiplier float64 `json:"multiplier"`
}

type PayoutResult struct {
	GameID           int                    `json:"game_id"`
	Hash             string                 `json:"hash"`
	ResultSide       Side                   `json:"result_side"`
	TotalBank        float64                `json:"total_bank"`
	TotalWinning     float64                `json:"total_winning"`
	HouseCut         float64                `json:"house_cut"`
	Distributable    float64                `json:"distributable"`
	HasWinners       bool                   `json:"has_winners"`
	Winners          map[int64]WinnerPayout `json:"winners"`
	HouseProfitTotal float64                `json:"house_profit_total"`
}

type Engine struct {
	mu sync.RWMutex

	phase      Phase
	timer      int
	gameID     int
	hash       string
	resultSide Side
	seedHex    string

	bets *BetStore
	cfg  *config.Config

	payouts map[int]PayoutResult
	history []PayoutResult

	houseProfitTotal float64
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

		bets:    NewBetStore(),
		cfg:     cfg,
		payouts: make(map[int]PayoutResult),
		history: make([]PayoutResult, 0),
	}
}

func (e *Engine) Snapshot() Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return Snapshot{
		Phase:      e.phase,
		Timer:      e.timer,
		GameID:     e.gameID,
		Hash:       e.hash,
		ResultSide: e.resultSide,
		Seed:       e.seedHex,
	}
}

func (e *Engine) Tick() (bool, Snapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()

	old := e.phase

	if e.timer > 0 {
		e.timer--
	}
	if e.timer == 0 {
		e.nextPhaseLocked()
	}

	return e.phase != old, e.snapshotLocked()
}

func (e *Engine) snapshotLocked() Snapshot {
	return Snapshot{
		Phase:      e.phase,
		Timer:      e.timer,
		GameID:     e.gameID,
		Hash:       e.hash,
		ResultSide: e.resultSide,
		Seed:       e.seedHex,
	}
}

func (e *Engine) nextPhaseLocked() {
	switch e.phase {

	case PhaseWaiting:
		if e.uniqueBetUsersLocked() < 2 {
			e.timer = -1
			return
		}
		e.phase = PhaseBetting
		e.timer = e.cfg.BettingTime
		log.Println("gameStarted")

	case PhaseBetting:
		e.phase = PhaseGettingResult
		e.timer = e.cfg.TimeTillResult

		seedBytes, err := hex.DecodeString(e.seedHex)
		if err != nil {
			e.resultSide = SideHeads
		} else {
			e.resultSide = Side(rng.SideFromSeed(seedBytes))
		}

		log.Println("gettingResult")

	case PhaseGettingResult:
		pr := e.calculatePayoutsLocked()
		e.payouts[e.gameID] = pr
		e.history = append(e.history, pr)
		if len(e.history) > 10 {
			e.history = e.history[1:]
		}
		log.Printf("=== PAYOUT DEBUG ===")
		log.Printf("GameID: %d", pr.GameID)
		log.Printf("TotalBank: %.6f", pr.TotalBank)
		log.Printf("TotalWinning: %.6f", pr.TotalWinning)
		log.Printf("HouseCut: %.6f", pr.HouseCut)
		log.Printf("Distributable: %.6f", pr.Distributable)
		log.Printf("HouseProfitTotal: %.6f", pr.HouseProfitTotal)
		log.Printf("HasWinners: %v", pr.HasWinners)

		for uid, w := range pr.Winners {
			log.Printf("Winner %d -> stake: %.6f payout: %.6f multiplier: %.6f",
				uid, w.Stake, w.Payout, w.Multiplier)
		}

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

	e.mu.Lock()
	defer e.mu.Unlock()

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
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.uniqueBetUsersLocked()
}

func (e *Engine) uniqueBetUsersLocked() int {
	return e.bets.UniqueUsers(e.gameID)
}

func (e *Engine) BetsSnapshot() any {
	e.mu.RLock()
	gid := e.gameID
	e.mu.RUnlock()

	snap := e.bets.Snapshot(gid)
	if len(snap) == 0 {
		return nil
	}
	return snap
}

func (e *Engine) TryStartFromWaiting() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.phase != PhaseWaiting {
		return false
	}
	if e.uniqueBetUsersLocked() < 2 {
		return false
	}
	e.phase = PhaseBetting
	e.timer = e.cfg.BettingTime
	log.Println("gameStarted")
	return true
}

func (e *Engine) PayoutForGame(gameID int) (PayoutResult, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pr, ok := e.payouts[gameID]
	return pr, ok
}

func (e *Engine) History() []PayoutResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := make([]PayoutResult, len(e.history))
	copy(out, e.history)
	return out
}
