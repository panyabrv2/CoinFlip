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

type GameRound struct {
	GameID           int64
	Phase            string
	Hash             string
	Seed             string
	ResultSide       *string
	CreatedAt        time.Time
	BettingStartedAt *time.Time
	ResultStartedAt  *time.Time
	FinishedAt       *time.Time
}

type GamesRepo struct {
	db *pgxpool.Pool
}

func NewGamesRepo(db *pgxpool.Pool) *GamesRepo {
	return &GamesRepo{db: db}
}

func (r *GamesRepo) NextGameID(ctx context.Context) (int, error) {
	const q = `
		SELECT COALESCE(MAX(game_id), 0) + 1
		FROM twist_business.game_rounds
	`

	var next int64
	if err := r.db.QueryRow(ctx, q).Scan(&next); err != nil {
		return 0, err
	}
	if next <= 0 {
		next = 1
	}
	return int(next), nil
}

func (r *GamesRepo) EnsureRound(ctx context.Context, gameID int, phase, hash, seed string) error {
	if gameID <= 0 {
		return fmt.Errorf("invalid game_id")
	}
	if phase == "" {
		return fmt.Errorf("empty phase")
	}
	if hash == "" {
		return fmt.Errorf("empty hash")
	}
	if seed == "" {
		return fmt.Errorf("empty seed")
	}

	const q = `
		INSERT INTO twist_business.game_rounds (
			game_id, phase, hash, seed
		)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (game_id) DO UPDATE SET
			phase = EXCLUDED.phase,
			hash  = EXCLUDED.hash,
			seed  = EXCLUDED.seed
	`
	_, err := r.db.Exec(ctx, q, gameID, phase, hash, seed)
	return err
}

func (r *GamesRepo) SetPhase(ctx context.Context, gameID int, phase string) error {
	if gameID <= 0 {
		return fmt.Errorf("invalid game_id")
	}
	if phase == "" {
		return fmt.Errorf("empty phase")
	}

	const q = `
		UPDATE twist_business.game_rounds
		SET
			phase = $2,
			betting_started_at = CASE
				WHEN $2 = 'betting' AND betting_started_at IS NULL THEN now()
				ELSE betting_started_at
			END,
			result_started_at = CASE
				WHEN $2 = 'gettingResult' AND result_started_at IS NULL THEN now()
				ELSE result_started_at
			END,
			finished_at = CASE
				WHEN $2 = 'finished' AND finished_at IS NULL THEN now()
				ELSE finished_at
			END
		WHERE game_id = $1
	`
	_, err := r.db.Exec(ctx, q, gameID, phase)
	return err
}

func (r *GamesRepo) FinishRound(ctx context.Context, gameID int, resultSide, seed string) error {
	if gameID <= 0 {
		return fmt.Errorf("invalid game_id")
	}
	if resultSide != "heads" && resultSide != "tails" {
		return fmt.Errorf("bad result_side")
	}
	if seed == "" {
		return fmt.Errorf("empty seed")
	}

	const q = `
		UPDATE twist_business.game_rounds
		SET
			phase = 'finished',
			result_side = $2,
			seed = $3,
			finished_at = now()
		WHERE game_id = $1
	`
	_, err := r.db.Exec(ctx, q, gameID, resultSide, seed)
	return err
}

func (r *GamesRepo) Get(ctx context.Context, gameID int) (*GameRound, error) {
	if gameID <= 0 {
		return nil, fmt.Errorf("invalid game_id")
	}

	const q = `
		SELECT
			game_id,
			phase,
			hash,
			seed,
			result_side,
			created_at,
			betting_started_at,
			result_started_at,
			finished_at
		FROM twist_business.game_rounds
		WHERE game_id = $1
	`

	row := r.db.QueryRow(ctx, q, gameID)

	var out GameRound
	var resultSide sql.NullString
	var bettingStartedAt sql.NullTime
	var resultStartedAt sql.NullTime
	var finishedAt sql.NullTime

	err := row.Scan(
		&out.GameID,
		&out.Phase,
		&out.Hash,
		&out.Seed,
		&resultSide,
		&out.CreatedAt,
		&bettingStartedAt,
		&resultStartedAt,
		&finishedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if resultSide.Valid {
		s := resultSide.String
		out.ResultSide = &s
	}
	if bettingStartedAt.Valid {
		t := bettingStartedAt.Time
		out.BettingStartedAt = &t
	}
	if resultStartedAt.Valid {
		t := resultStartedAt.Time
		out.ResultStartedAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		out.FinishedAt = &t
	}

	return &out, nil
}
