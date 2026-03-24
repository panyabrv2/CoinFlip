package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WalletsRepo struct {
	db *pgxpool.Pool
}

func NewWalletsRepo(db *pgxpool.Pool) *WalletsRepo {
	return &WalletsRepo{db: db}
}

func (r *WalletsRepo) EnsureWallet(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user_id")
	}

	const q = `
		INSERT INTO twist_business.user_wallets (user_id, balance_ton)
		VALUES ($1, 0)
		ON CONFLICT (user_id) DO NOTHING
	`
	_, err := r.db.Exec(ctx, q, userID)
	return err
}
