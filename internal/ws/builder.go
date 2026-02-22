package ws

import "CoinFlip/internal/game"

func EventForPhase(s game.Snapshot) any {
	switch s.Phase {
	case game.PhaseBetting:
		return GameStarted{
			Event:       EventGameStarted,
			GameID:      s.GameID,
			Hash:        s.Hash,
			BettingTime: s.Timer,
		}

	case game.PhaseGettingResult:
		return GettingResult{
			Event:          EventGettingResult,
			GameID:         s.GameID,
			Hash:           s.Hash,
			TimeTillResult: s.Timer,
			ResultSide:     string(s.ResultSide),
		}

	case game.PhaseFinished:
		return GameFinished{
			Event:      EventGameFinished,
			GameID:     s.GameID,
			Hash:       s.Hash,
			ResultSide: string(s.ResultSide),
		}

	case game.PhaseWaiting:
		return NewGame{
			Event:  EventNewGame,
			GameID: s.GameID,
			Hash:   s.Hash,
		}

	default:
		return nil
	}
}
