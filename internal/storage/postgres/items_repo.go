package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")
var ErrAlreadyLocked = errors.New("already locked or not owned")
var ErrPartialLock = errors.New("partial lock: some items not found/not owned/already locked")

type Item struct {
	ItemID   int
	UserID   int64
	Name     string
	PhotoURL *string
	CostTon  float64
	Type     string
	Locked   bool
}

type ItemsRepo struct {
	db *pgxpool.Pool
}

func NewItemsRepo(db *pgxpool.Pool) *ItemsRepo {
	return &ItemsRepo{db: db}
}

func (r *ItemsRepo) LockItem(ctx context.Context, itemID int, userID int64) (*Item, error) {
	const q = `
		UPDATE twist_business.items
		SET locked = true
		WHERE item_id = $1
		  AND user_id = $2
		  AND locked = false
		RETURNING item_id, user_id, name, photo_url, cost_ton, type, locked
	`

	row := r.db.QueryRow(ctx, q, itemID, userID)

	var it Item
	if err := row.Scan(&it.ItemID, &it.UserID, &it.Name, &it.PhotoURL, &it.CostTon, &it.Type, &it.Locked); err != nil {
		return nil, ErrAlreadyLocked
	}
	return &it, nil
}

func (r *ItemsRepo) LockItems(ctx context.Context, itemIDs []int, userID int64) ([]Item, error) {
	if len(itemIDs) == 0 {
		return nil, fmt.Errorf("empty itemIDs")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const q = `
		UPDATE twist_business.items
		SET locked = true
		WHERE item_id = ANY($1)
		  AND user_id = $2
		  AND locked = false
		RETURNING item_id, user_id, name, photo_url, cost_ton, type, locked
	`

	rows, err := tx.Query(ctx, q, itemIDs, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Item, 0, len(itemIDs))
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ItemID, &it.UserID, &it.Name, &it.PhotoURL, &it.CostTon, &it.Type, &it.Locked); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(out) != len(itemIDs) {
		return nil, ErrPartialLock
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return out, nil
}

func (r *ItemsRepo) UnlockItem(ctx context.Context, itemID int) error {
	const q = `
		UPDATE twist_business.items
		SET locked = false
		WHERE item_id = $1
	`
	_, err := r.db.Exec(ctx, q, itemID)
	return err
}

func (r *ItemsRepo) UnlockItems(ctx context.Context, itemIDs []int) error {
	if len(itemIDs) == 0 {
		return nil
	}
	const q = `
		UPDATE twist_business.items
		SET locked = false
		WHERE item_id = ANY($1)
	`
	_, err := r.db.Exec(ctx, q, itemIDs)
	return err
}

func (r *ItemsRepo) ConsumeLockedItems(ctx context.Context, itemIDs []int) (int64, error) {
	if len(itemIDs) == 0 {
		return 0, nil
	}

	const q = `
		DELETE FROM twist_business.items
		WHERE item_id = ANY($1)
		  AND locked = true
	`

	tag, err := r.db.Exec(ctx, q, itemIDs)
	if err != nil {
		return 0, err
	}

	return tag.RowsAffected(), nil
}
