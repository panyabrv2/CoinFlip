package game

const singleMultiplier = 1.96

type UserSingleResult struct {
	UserID     int64   `json:"user_id"`
	Stake      float64 `json:"stake"`
	Payout     float64 `json:"payout"`
	Multiplier float64 `json:"multiplier"`
	Win        bool    `json:"win"`
}

type PayoutResult struct {
	GameID     int                        `json:"game_id"`
	Hash       string                     `json:"hash"`
	ResultSide Side                       `json:"result_side"`
	Results    map[int64]UserSingleResult `json:"results"`
}

func betStakeValue(b BetSnapshot) float64 {
	if b.BetItem.CostTon > 0 {
		return b.BetItem.CostTon
	}
	return 1.0
}

func (e *Engine) calculatePayoutsLocked() PayoutResult {
	gid := e.gameID
	result := e.resultSide

	out := PayoutResult{
		GameID:     gid,
		Hash:       e.hash,
		ResultSide: result,
		Results:    make(map[int64]UserSingleResult),
	}

	snap := e.bets.Snapshot(gid)

	for _, ub := range snap {
		for _, b := range ub.Bets {
			if b.Mode != "single" {
				continue
			}

			stake := betStakeValue(b)

			r := out.Results[b.UserID]
			r.UserID = b.UserID
			r.Stake += stake

			if Side(b.Side) == result {
				r.Win = true
				r.Payout += stake * singleMultiplier
				r.Multiplier = singleMultiplier
			}

			out.Results[b.UserID] = r
		}
	}

	return out
}
