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

type SeriesStage string

const (
	SeriesStageInRound        SeriesStage = "in_round"
	SeriesStageAwaitingChoice SeriesStage = "awaiting_choice"
)

type SeriesSnapshot struct {
	UserID      int64       `json:"user_id"`
	Side        string      `json:"side"`
	Stake       float64     `json:"stake"`
	Wins        int         `json:"wins"`
	Multiplier  float64     `json:"multiplier"`
	Claimable   float64     `json:"claimable"`
	Stage       SeriesStage `json:"stage"`
	RoundGameID int         `json:"-"`
	Active      bool        `json:"active"`
}

type SeriesState struct {
	UserID      int64
	Side        string
	Stake       float64
	Wins        int
	Multiplier  float64
	Stage       SeriesStage
	RoundGameID int
	Active      bool
}

type SeriesRoundResult struct {
	GameID     int         `json:"game_id"`
	UserID     int64       `json:"user_id"`
	Side       string      `json:"side"`
	Stake      float64     `json:"stake"`
	Wins       int         `json:"wins"`
	Multiplier float64     `json:"multiplier"`
	Claimable  float64     `json:"claimable"`
	Stage      SeriesStage `json:"stage"`
	Active     bool        `json:"active"`
	Outcome    string      `json:"outcome"`
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

	series        map[int64]*SeriesState
	seriesResults map[int]map[int64]SeriesRoundResult
}

func newRoundSeed() (string, string) {
	seedBytes, err := rng.NewSeed()
	if err != nil {
		seedBytes = []byte("fallback-seed-fallback-seed-123456")
	}

	seedHex := hex.EncodeToString(seedBytes)
	hash := rng.SHA256Hex(seedBytes)
	return seedHex, hash
}

func claimableForSeries(s *SeriesState) float64 {
	if s == nil || !s.Active || s.Wins == 0 || s.Multiplier <= 1.0 {
		return 0
	}
	return s.Stake * s.Multiplier
}

func NewEngine(cfg *config.Config, startGameID int) *Engine {
	if startGameID <= 0 {
		startGameID = 1
	}

	seedHex, hash := newRoundSeed()

	return &Engine{
		phase:   PhaseWaiting,
		timer:   -1,
		gameID:  startGameID,
		seedHex: seedHex,
		hash:    hash,

		bets:    NewBetStore(),
		cfg:     cfg,
		payouts: make(map[int]PayoutResult),
		history: make([]PayoutResult, 0),

		series:        make(map[int64]*SeriesState),
		seriesResults: make(map[int]map[int64]SeriesRoundResult),
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

func (e *Engine) SeriesSnapshot(userID int64) (*SeriesSnapshot, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	s, ok := e.series[userID]
	if !ok || s == nil {
		return nil, false
	}

	return &SeriesSnapshot{
		UserID:      s.UserID,
		Side:        s.Side,
		Stake:       s.Stake,
		Wins:        s.Wins,
		Multiplier:  s.Multiplier,
		Claimable:   claimableForSeries(s),
		Stage:       s.Stage,
		RoundGameID: s.RoundGameID,
		Active:      s.Active,
	}, true
}

func (e *Engine) RestoreSeriesSnapshot(ss SeriesSnapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !ss.Active {
		delete(e.series, ss.UserID)
		return
	}

	e.series[ss.UserID] = &SeriesState{
		UserID:      ss.UserID,
		Side:        ss.Side,
		Stake:       ss.Stake,
		Wins:        ss.Wins,
		Multiplier:  ss.Multiplier,
		Stage:       ss.Stage,
		RoundGameID: ss.RoundGameID,
		Active:      ss.Active,
	}
}

func (e *Engine) Tick(hasOnline bool) (bool, Snapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()

	old := e.phase

	if e.phase == PhaseWaiting {
		if hasOnline {
			e.phase = PhaseBetting
			e.timer = e.cfg.BettingTime
			log.Printf("game: phase from=waiting to=betting game_id=%d timer=%d", e.gameID, e.timer)
		}
		return e.phase != old, e.snapshotLocked()
	}

	if e.timer > 0 {
		e.timer--
	}
	if e.timer == 0 {
		e.nextPhaseLocked(hasOnline)
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

func (e *Engine) nextPhaseLocked(hasOnline bool) {
	switch e.phase {
	case PhaseBetting:
		e.phase = PhaseGettingResult
		e.timer = e.cfg.TimeTillResult

		seedBytes, err := hex.DecodeString(e.seedHex)
		if err != nil {
			e.resultSide = SideHeads
		} else {
			e.resultSide = Side(rng.SideFromSeed(seedBytes))
		}

		log.Printf("game: phase from=betting to=gettingResult game_id=%d timer=%d", e.gameID, e.timer)

	case PhaseGettingResult:
		seriesRes := e.updateSeriesLocked()
		if len(seriesRes) > 0 {
			e.seriesResults[e.gameID] = seriesRes
		}

		pr := e.calculatePayoutsLocked()
		e.payouts[e.gameID] = pr

		e.history = append(e.history, pr)
		if len(e.history) > 10 {
			oldGameID := e.history[0].GameID
			e.history = e.history[1:]
			delete(e.payouts, oldGameID)
			delete(e.seriesResults, oldGameID)
		}

		e.phase = PhaseFinished
		e.timer = e.cfg.NextGameDelay
		log.Printf("game: phase from=gettingResult to=finished game_id=%d timer=%d result=%s", e.gameID, e.timer, e.resultSide)

	case PhaseFinished:
		finishedGameID := e.gameID
		e.bets.Reset(finishedGameID)

		e.gameID++
		e.resultSide = Side("")
		e.seedHex, e.hash = newRoundSeed()

		if !hasOnline {
			e.phase = PhaseWaiting
			e.timer = -1
			log.Printf("game: phase from=finished to=waiting game_id=%d", e.gameID)
			return
		}

		e.phase = PhaseBetting
		e.timer = e.cfg.BettingTime
		log.Printf("game: phase from=finished to=betting game_id=%d timer=%d", e.gameID, e.timer)
	}
}

func (e *Engine) AddBet(userID int64, side string, mode string, items []ItemRef) (Snapshot, int, bool, string) {
	if userID == 0 {
		return Snapshot{}, 0, false, "bad user_id"
	}
	if side != string(SideHeads) && side != string(SideTails) {
		return Snapshot{}, 0, false, "bad side"
	}
	if len(items) == 0 {
		return Snapshot{}, 0, false, "empty items"
	}

	mode = "series"

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.phase != PhaseBetting || e.timer <= 0 {
		return e.snapshotLocked(), 0, false, "betting closed"
	}

	if s, exists := e.series[userID]; exists && s != nil && s.Active {
		return e.snapshotLocked(), 0, false, "active series already exists"
	}

	accepted := e.bets.Add(e.gameID, userID, side, mode, items)

	stake := 0.0
	for _, it := range items {
		stake += it.CostTon
	}

	if stake > 0 {
		e.series[userID] = &SeriesState{
			UserID:      userID,
			Side:        side,
			Stake:       stake,
			Wins:        0,
			Multiplier:  1.0,
			Stage:       SeriesStageInRound,
			RoundGameID: e.gameID,
			Active:      true,
		}
	}

	return e.snapshotLocked(), accepted, true, ""
}

func (e *Engine) RollbackAcceptedBet(gameID int, userID int64, mode string, itemCount int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if gameID <= 0 || userID <= 0 || itemCount <= 0 {
		return
	}

	e.bets.RemoveLastN(gameID, userID, itemCount)

	s, ok := e.series[userID]
	if ok && s != nil && s.Active && s.Wins == 0 && s.RoundGameID == gameID && s.Stage == SeriesStageInRound {
		delete(e.series, userID)
	}
}

func (e *Engine) SeriesContinue(userID int64, side string) (*SeriesSnapshot, bool, string) {
	if userID == 0 {
		return nil, false, "bad user_id"
	}
	if side != string(SideHeads) && side != string(SideTails) {
		return nil, false, "bad side"
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.phase != PhaseBetting || e.timer <= 0 {
		return nil, false, "betting closed"
	}

	s, exists := e.series[userID]
	if !exists || s == nil || !s.Active {
		return nil, false, "no active series"
	}

	if s.Stage != SeriesStageAwaitingChoice {
		return nil, false, "series is already participating in current round"
	}

	s.Side = side
	s.Stage = SeriesStageInRound
	s.RoundGameID = e.gameID

	out := &SeriesSnapshot{
		UserID:      s.UserID,
		Side:        s.Side,
		Stake:       s.Stake,
		Wins:        s.Wins,
		Multiplier:  s.Multiplier,
		Claimable:   claimableForSeries(s),
		Stage:       s.Stage,
		RoundGameID: s.RoundGameID,
		Active:      s.Active,
	}

	return out, true, ""
}

func (e *Engine) Cashout(userID int64) (stake float64, multiplier float64, payout float64, ok bool, reason string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.phase != PhaseBetting || e.timer <= 0 {
		return 0, 0, 0, false, "cashout allowed only in betting"
	}

	s, exists := e.series[userID]
	if !exists || !s.Active {
		return 0, 0, 0, false, "no active series"
	}

	if s.Stage != SeriesStageAwaitingChoice {
		return 0, 0, 0, false, "series is not waiting for choice"
	}

	if s.Wins == 0 || s.Multiplier <= 1.0 {
		return 0, 0, 0, false, "series has no claimable win yet"
	}

	stake = s.Stake
	multiplier = s.Multiplier
	payout = stake * multiplier

	delete(e.series, userID)

	return stake, multiplier, payout, true, ""
}

func (e *Engine) BetsSnapshot() any {
	e.mu.RLock()
	gid := e.gameID
	e.mu.RUnlock()

	return e.BetsSnapshotForGame(gid)
}

func (e *Engine) BetsSnapshotForGame(gameID int) any {
	snap := e.bets.Snapshot(gameID)
	if len(snap) == 0 {
		return nil
	}
	return snap
}

func (e *Engine) PayoutForGame(gameID int) (PayoutResult, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pr, ok := e.payouts[gameID]
	return pr, ok
}

func (e *Engine) SeriesResultsForGame(gameID int) (map[int64]SeriesRoundResult, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	src, ok := e.seriesResults[gameID]
	if !ok {
		return nil, false
	}

	out := make(map[int64]SeriesRoundResult, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out, true
}

func (e *Engine) History() []PayoutResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := make([]PayoutResult, len(e.history))
	copy(out, e.history)
	return out
}

func (e *Engine) updateSeriesLocked() map[int64]SeriesRoundResult {
	results := make(map[int64]SeriesRoundResult)

	for userID, s := range e.series {
		if !s.Active {
			continue
		}
		if s.Stage != SeriesStageInRound {
			continue
		}
		if s.RoundGameID != e.gameID {
			continue
		}

		playedSide := s.Side

		if Side(playedSide) != e.resultSide {
			results[userID] = SeriesRoundResult{
				GameID:     e.gameID,
				UserID:     userID,
				Side:       playedSide,
				Stake:      s.Stake,
				Wins:       s.Wins,
				Multiplier: s.Multiplier,
				Claimable:  0,
				Stage:      "",
				Active:     false,
				Outcome:    "lose",
			}

			delete(e.series, userID)
			log.Printf("series lost user=%d", userID)
			continue
		}

		s.Wins++
		if s.Wins == 1 {
			s.Multiplier = 1.96
		} else {
			s.Multiplier = s.Multiplier * 2
		}

		s.Stage = SeriesStageAwaitingChoice
		s.RoundGameID = 0
		s.Side = ""

		results[userID] = SeriesRoundResult{
			GameID:     e.gameID,
			UserID:     userID,
			Side:       playedSide,
			Stake:      s.Stake,
			Wins:       s.Wins,
			Multiplier: s.Multiplier,
			Claimable:  claimableForSeries(s),
			Stage:      s.Stage,
			Active:     true,
			Outcome:    "win",
		}

		log.Printf("series win user=%d wins=%d multiplier=%.2f", userID, s.Wins, s.Multiplier)
	}

	return results
}
