package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UsersRepo struct {
	db *pgxpool.Pool
}

func NewUsersRepo(db *pgxpool.Pool) *UsersRepo {
	return &UsersRepo{db: db}
}

func (r *UsersRepo) EnsureUser(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user_id")
	}

	const q = `
		INSERT INTO twist_business.users (user_id, username, registration_date)
		VALUES ($1, NULL, now())
		ON CONFLICT (user_id) DO NOTHING
	`
	_, err := r.db.Exec(ctx, q, userID)
	return err
}
