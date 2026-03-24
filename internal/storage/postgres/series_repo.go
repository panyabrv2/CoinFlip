package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrActiveSeriesNotFound = errors.New("active series not found")

type SeriesSession struct {
	ID                 int64
	UserID             int64
	InitialGameID      int64
	Active             bool
	Stage              string
	RoundGameID        *int64
	CurrentSide        *string
	StakeTon           float64
	Wins               int
	Multiplier         float64
	ClaimableTon       float64
	CashedOutPayoutTon *float64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ClosedAt           *time.Time
}

type CreateSeriesSessionParams struct {
	UserID        int64
	InitialGameID int
	CurrentSide   string
	StakeTon      float64
}

type SeriesRepo struct {
	db *pgxpool.Pool
}

func NewSeriesRepo(db *pgxpool.Pool) *SeriesRepo {
	return &SeriesRepo{db: db}
}

func (r *SeriesRepo) CreateSession(ctx context.Context, p CreateSeriesSessionParams) (int64, error) {
	if p.UserID <= 0 {
		return 0, fmt.Errorf("invalid user_id")
	}
	if p.InitialGameID <= 0 {
		return 0, fmt.Errorf("invalid initial_game_id")
	}
	if p.CurrentSide != "heads" && p.CurrentSide != "tails" {
		return 0, fmt.Errorf("bad current_side")
	}
	if p.StakeTon <= 0 {
		return 0, fmt.Errorf("invalid stake_ton")
	}

	const q = `
		INSERT INTO twist_business.series_sessions (
			user_id,
			initial_game_id,
			active,
			stage,
			round_game_id,
			current_side,
			stake_ton,
			wins,
			multiplier,
			claimable_ton
		)
		VALUES (
			$1, $2, TRUE, 'in_round', $2, $3, $4, 0, 1.0, 0
		)
		RETURNING id
	`

	var sessionID int64
	err := r.db.QueryRow(ctx, q, p.UserID, p.InitialGameID, p.CurrentSide, p.StakeTon).Scan(&sessionID)
	return sessionID, err
}

func (r *SeriesRepo) DeleteSession(ctx context.Context, sessionID int64) error {
	if sessionID <= 0 {
		return fmt.Errorf("invalid session_id")
	}

	const q = `
		DELETE FROM twist_business.series_sessions
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, q, sessionID)
	return err
}

func (r *SeriesRepo) GetActiveByUser(ctx context.Context, userID int64) (*SeriesSession, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}

	const q = `
		SELECT
			id,
			user_id,
			initial_game_id,
			active,
			stage,
			round_game_id,
			current_side,
			stake_ton,
			wins,
			multiplier,
			claimable_ton,
			cashed_out_payout_ton,
			created_at,
			updated_at,
			closed_at
		FROM twist_business.series_sessions
		WHERE user_id = $1
		  AND active = TRUE
	`

	row := r.db.QueryRow(ctx, q, userID)
	return scanSeriesSession(row)
}

func (r *SeriesRepo) Continue(ctx context.Context, userID int64, gameID int, side string) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user_id")
	}
	if gameID <= 0 {
		return fmt.Errorf("invalid game_id")
	}
	if side != "heads" && side != "tails" {
		return fmt.Errorf("bad side")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	s, err := r.getActiveForUpdate(ctx, tx, userID)
	if err != nil {
		return err
	}
	if s.Stage != "awaiting_choice" {
		return fmt.Errorf("series is not awaiting_choice")
	}

	const q = `
		UPDATE twist_business.series_sessions
		SET
			stage = 'in_round',
			round_game_id = $2,
			current_side = $3,
			updated_at = now()
		WHERE id = $1
	`
	if _, err := tx.Exec(ctx, q, s.ID, gameID, side); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *SeriesRepo) MoveToAwaitingChoiceAfterWin(
	ctx context.Context,
	userID int64,
	gameID int,
	playedSide string,
	wins int,
	multiplier float64,
	claimable float64,
) (int64, error) {
	if userID <= 0 {
		return 0, fmt.Errorf("invalid user_id")
	}
	if gameID <= 0 {
		return 0, fmt.Errorf("invalid game_id")
	}
	if playedSide != "heads" && playedSide != "tails" {
		return 0, fmt.Errorf("bad played_side")
	}
	if wins <= 0 {
		return 0, fmt.Errorf("invalid wins")
	}
	if multiplier <= 1.0 {
		return 0, fmt.Errorf("invalid multiplier")
	}
	if claimable <= 0 {
		return 0, fmt.Errorf("invalid claimable")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	s, err := r.getActiveForUpdate(ctx, tx, userID)
	if err != nil {
		return 0, err
	}

	const uq = `
		UPDATE twist_business.series_sessions
		SET
			stage = 'awaiting_choice',
			round_game_id = NULL,
			current_side = NULL,
			wins = $2,
			multiplier = $3,
			claimable_ton = $4,
			updated_at = now()
		WHERE id = $1
	`
	if _, err := tx.Exec(ctx, uq, s.ID, wins, multiplier, claimable); err != nil {
		return 0, err
	}

	const iq = `
		INSERT INTO twist_business.series_steps (
			session_id,
			game_id,
			event,
			chosen_side,
			wins_after,
			multiplier_after,
			claimable_after
		)
		VALUES ($1, $2, 'win', $3, $4, $5, $6)
	`
	if _, err := tx.Exec(ctx, iq, s.ID, gameID, playedSide, wins, multiplier, claimable); err != nil {
		return 0, err
	}

	const bq = `
		UPDATE twist_business.game_bets
		SET status = 'series_awaiting_choice'
		WHERE series_session_id = $1
		  AND status = 'accepted'
	`
	if _, err := tx.Exec(ctx, bq, s.ID); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return s.ID, nil
}

func (r *SeriesRepo) MarkLost(
	ctx context.Context,
	userID int64,
	gameID int,
	playedSide string,
	wins int,
	multiplier float64,
) (int64, error) {
	if userID <= 0 {
		return 0, fmt.Errorf("invalid user_id")
	}
	if gameID <= 0 {
		return 0, fmt.Errorf("invalid game_id")
	}
	if playedSide != "heads" && playedSide != "tails" {
		return 0, fmt.Errorf("bad played_side")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	s, err := r.getActiveForUpdate(ctx, tx, userID)
	if err != nil {
		return 0, err
	}

	const uq = `
		UPDATE twist_business.series_sessions
		SET
			active = FALSE,
			stage = 'lost',
			round_game_id = NULL,
			current_side = NULL,
			claimable_ton = 0,
			updated_at = now(),
			closed_at = now()
		WHERE id = $1
	`
	if _, err := tx.Exec(ctx, uq, s.ID); err != nil {
		return 0, err
	}

	const iq = `
		INSERT INTO twist_business.series_steps (
			session_id,
			game_id,
			event,
			chosen_side,
			wins_after,
			multiplier_after,
			claimable_after
		)
		VALUES ($1, $2, 'lose', $3, $4, $5, 0)
	`
	if _, err := tx.Exec(ctx, iq, s.ID, gameID, playedSide, wins, multiplier); err != nil {
		return 0, err
	}

	const bq = `
		UPDATE twist_business.game_bets
		SET
			status = 'series_lost',
			payout_ton = 0,
			settled_at = now()
		WHERE series_session_id = $1
		  AND status IN ('accepted', 'series_awaiting_choice')
	`
	if _, err := tx.Exec(ctx, bq, s.ID); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return s.ID, nil
}

func (r *SeriesRepo) Cashout(ctx context.Context, userID int64, gameID int, payout float64) (int64, error) {
	if userID <= 0 {
		return 0, fmt.Errorf("invalid user_id")
	}
	if gameID <= 0 {
		return 0, fmt.Errorf("invalid game_id")
	}
	if payout <= 0 {
		return 0, fmt.Errorf("invalid payout")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	s, err := r.getActiveForUpdate(ctx, tx, userID)
	if err != nil {
		return 0, err
	}
	if s.Stage != "awaiting_choice" {
		return 0, fmt.Errorf("series is not awaiting_choice")
	}

	const ensureWalletQ = `
		INSERT INTO twist_business.user_wallets (user_id, balance_ton)
		VALUES ($1, 0)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, ensureWalletQ, userID); err != nil {
		return 0, err
	}

	const uq = `
		UPDATE twist_business.series_sessions
		SET
			active = FALSE,
			stage = 'cashed_out',
			round_game_id = NULL,
			current_side = NULL,
			cashed_out_payout_ton = $2,
			updated_at = now(),
			closed_at = now()
		WHERE id = $1
	`
	if _, err := tx.Exec(ctx, uq, s.ID, payout); err != nil {
		return 0, err
	}

	const iq = `
		INSERT INTO twist_business.series_steps (
			session_id,
			game_id,
			event,
			chosen_side,
			wins_after,
			multiplier_after,
			claimable_after
		)
		VALUES ($1, $2, 'cashout', NULL, $3, $4, $5)
	`
	if _, err := tx.Exec(ctx, iq, s.ID, gameID, s.Wins, s.Multiplier, payout); err != nil {
		return 0, err
	}

	const bq = `
		WITH total AS (
			SELECT COALESCE(SUM(stake_ton), 0) AS total_stake
			FROM twist_business.game_bets
			WHERE series_session_id = $1
		)
		UPDATE twist_business.game_bets AS b
		SET
			status = 'series_cashed_out',
			payout_ton = CASE
				WHEN total.total_stake > 0
					THEN ROUND(($2::numeric * b.stake_ton / total.total_stake), 8)
				ELSE 0
			END,
			settled_at = now()
		FROM total
		WHERE b.series_session_id = $1
		  AND b.status IN ('accepted', 'series_awaiting_choice')
	`
	if _, err := tx.Exec(ctx, bq, s.ID, payout); err != nil {
		return 0, err
	}

	const creditQ = `
		WITH ins AS (
			INSERT INTO twist_business.wallet_transactions (
				user_id,
				game_id,
				series_session_id,
				kind,
				amount_ton
			)
			VALUES ($1, $2, $3, 'series_cashout', $4)
			ON CONFLICT DO NOTHING
			RETURNING user_id, amount_ton
		)
		UPDATE twist_business.user_wallets uw
		SET
			balance_ton = uw.balance_ton + ins.amount_ton,
			updated_at = now()
		FROM ins
		WHERE uw.user_id = ins.user_id
	`
	if _, err := tx.Exec(ctx, creditQ, userID, gameID, s.ID, payout); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return s.ID, nil
}

func (r *SeriesRepo) getActiveForUpdate(ctx context.Context, tx pgx.Tx, userID int64) (*SeriesSession, error) {
	const q = `
		SELECT
			id,
			user_id,
			initial_game_id,
			active,
			stage,
			round_game_id,
			current_side,
			stake_ton,
			wins,
			multiplier,
			claimable_ton,
			cashed_out_payout_ton,
			created_at,
			updated_at,
			closed_at
		FROM twist_business.series_sessions
		WHERE user_id = $1
		  AND active = TRUE
		FOR UPDATE
	`

	row := tx.QueryRow(ctx, q, userID)
	return scanSeriesSession(row)
}

func scanSeriesSession(row pgx.Row) (*SeriesSession, error) {
	var out SeriesSession

	var roundGameID sql.NullInt64
	var currentSide sql.NullString
	var cashedOut sql.NullFloat64
	var closedAt sql.NullTime

	err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.InitialGameID,
		&out.Active,
		&out.Stage,
		&roundGameID,
		&currentSide,
		&out.StakeTon,
		&out.Wins,
		&out.Multiplier,
		&out.ClaimableTon,
		&cashedOut,
		&out.CreatedAt,
		&out.UpdatedAt,
		&closedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrActiveSeriesNotFound
		}
		return nil, err
	}

	if roundGameID.Valid {
		v := roundGameID.Int64
		out.RoundGameID = &v
	}
	if currentSide.Valid {
		v := currentSide.String
		out.CurrentSide = &v
	}
	if cashedOut.Valid {
		v := cashedOut.Float64
		out.CashedOutPayoutTon = &v
	}
	if closedAt.Valid {
		v := closedAt.Time
		out.ClosedAt = &v
	}

	return &out, nil
}