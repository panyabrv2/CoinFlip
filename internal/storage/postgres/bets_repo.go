package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateBetRow struct {
	GameID          int
	UserID          int64
	Side            string
	Mode            string
	SeriesSessionID *int64
	ItemID          int
	ItemType        string
	ItemName        string
	ItemPhotoURL    *string
	StakeTon        float64
}

type UserPayout struct {
	UserID    int64
	AmountTon float64
}

type BetsRepo struct {
	db *pgxpool.Pool
}

func NewBetsRepo(db *pgxpool.Pool) *BetsRepo {
	return &BetsRepo{db: db}
}

func (r *BetsRepo) InsertAcceptedBets(ctx context.Context, rows []CreateBetRow) error {
	if len(rows) == 0 {
		return nil
	}

	const q = `
		INSERT INTO twist_business.game_bets (
			game_id,
			user_id,
			side,
			mode,
			series_session_id,
			item_id,
			item_type,
			item_name,
			item_photo_url,
			stake_ton,
			status,
			payout_ton
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			'accepted', 0
		)
	`

	batch := &pgx.Batch{}
	for _, row := range rows {
		if row.GameID <= 0 {
			return fmt.Errorf("invalid game_id")
		}
		if row.UserID <= 0 {
			return fmt.Errorf("invalid user_id")
		}
		if row.Side != "heads" && row.Side != "tails" {
			return fmt.Errorf("bad side")
		}
		if row.Mode != "series" {
			return fmt.Errorf("bad mode")
		}
		if row.ItemID <= 0 {
			return fmt.Errorf("invalid item_id")
		}
		if row.ItemType == "" {
			return fmt.Errorf("empty item_type")
		}
		if row.ItemName == "" {
			return fmt.Errorf("empty item_name")
		}
		if row.StakeTon <= 0 {
			return fmt.Errorf("invalid stake_ton")
		}

		batch.Queue(
			q,
			row.GameID,
			row.UserID,
			row.Side,
			row.Mode,
			row.SeriesSessionID,
			row.ItemID,
			row.ItemType,
			row.ItemName,
			row.ItemPhotoURL,
			row.StakeTon,
		)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	for range rows {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}

	return nil
}

func (r *BetsRepo) ItemIDsForGame(ctx context.Context, gameID int) ([]int, error) {
	if gameID <= 0 {
		return nil, fmt.Errorf("invalid game_id")
	}

	const q = `
		SELECT DISTINCT item_id
		FROM twist_business.game_bets
		WHERE game_id = $1
		ORDER BY item_id
	`

	rows, err := r.db.Query(ctx, q, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]int, 0)
	for rows.Next() {
		var itemID int64
		if err := rows.Scan(&itemID); err != nil {
			return nil, err
		}
		out = append(out, int(itemID))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}
