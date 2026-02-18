package ws

import (
	"CoinFlip/internal/game"
	"time"
)

func endsAt(now time.Time, seconds int) string {
	return now.Add(time.Duration(seconds) * time.Second).UTC().Format(time.RFC3339)
}

func EventForPhase(s game.Snapshot, now time.Time) any {
	serverTime := now.UTC().Format(time.RFC3339)
	switch s.Phase {
	case game.PhaseBetting:
		return GameStarted{
			Event:       EventGameStarted,
			GameID:      s.GameID,
			Hash:        s.Hash,
			BettingTime: s.Timer,
			EndsAt:      endsAt(now, s.Timer),
			ServerTime:  serverTime,
		}

	case game.PhaseGettingResult:
		return GettingResult{
			Event:          EventGettingResult,
			GameID:         s.GameID,
			Hash:           s.Hash,
			TimeTillResult: s.Timer,
			ResultSide:     string(s.ResultSide),
			EndsAt:         endsAt(now, s.Timer),
			ServerTime:     serverTime,
		}

	case game.PhaseFinished:
		return GameFinished{
			Event:      EventGameFinished,
			GameID:     s.GameID,
			Hash:       s.Hash,
			ResultSide: string(s.ResultSide),
			Seed:       s.Seed,
			ServerTime: serverTime,
		}

	case game.PhaseWaiting:
		return NewGame{
			Event:      EventNewGame,
			GameID:     s.GameID,
			Hash:       s.Hash,
			ServerTime: serverTime,
		}
	default:
		return nil
	}
}
