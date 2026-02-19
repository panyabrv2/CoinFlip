package game

func betStakeValue(b BetSnapshot) float64 {
	if b.BetItem.CostTon > 0 {
		return b.BetItem.CostTon
	}
	return 1.0
}

func (e *Engine) calculatePayoutsLocked() PayoutResult {
	gid := e.gameID
	result := e.resultSide

	var totalBank float64
	var totalWinning float64
	perWinnerStake := make(map[int64]float64)

	snap := e.bets.Snapshot(gid)
	for _, ub := range snap {
		for _, b := range ub.Bets {
			v := betStakeValue(b)

			totalBank += v

			if Side(b.Side) == result {
				totalWinning += v
				perWinnerStake[b.UserID] += v
			}
		}
	}

	houseCut := totalBank * e.cfg.HouseEdge
	distributable := totalBank - houseCut

	out := PayoutResult{
		GameID:           gid,
		Hash:             e.hash,
		ResultSide:       result,
		TotalBank:        totalBank,
		TotalWinning:     totalWinning,
		Distributable:    distributable,
		HouseCut:         houseCut,
		HasWinners:       totalWinning > 0,
		Winners:          make(map[int64]WinnerPayout),
		HouseProfitTotal: 0,
	}

	e.houseProfitTotal += houseCut
	out.HouseProfitTotal = e.houseProfitTotal

	if totalWinning <= 0 || totalBank <= 0 {
		return out
	}

	for uid, stake := range perWinnerStake {
		payout := distributable * (stake / totalWinning)

		mult := 0.0
		if stake > 0 {
			mult = payout / stake
		}

		out.Winners[uid] = WinnerPayout{
			UserID:     uid,
			Stake:      stake,
			Payout:     payout,
			Multiplier: mult,
		}
	}

	return out
}
