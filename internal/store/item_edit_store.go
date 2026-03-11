package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/vbonduro/kitchinv/internal/domain"
)

type ItemEditStore struct {
	db *sql.DB
}

func NewItemEditStore(db *sql.DB) *ItemEditStore {
	return &ItemEditStore{db: db}
}

func (s *ItemEditStore) Create(ctx context.Context, itemID int64, field, oldValue, newValue string) (*domain.ItemEdit, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO item_edits (item_id, field, old_value, new_value) VALUES (?, ?, ?, ?)
	`, itemID, field, oldValue, newValue)
	if err != nil {
		return nil, fmt.Errorf("failed to create item edit: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	edit := &domain.ItemEdit{}
	err = s.db.QueryRowContext(ctx, `
		SELECT id, item_id, field, old_value, new_value, edited_at FROM item_edits WHERE id = ?
	`, id).Scan(&edit.ID, &edit.ItemID, &edit.Field, &edit.OldValue, &edit.NewValue, &edit.EditedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created edit: %w", err)
	}

	return edit, nil
}

func (s *ItemEditStore) ListByItemID(ctx context.Context, itemID int64) ([]*domain.ItemEdit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, item_id, field, old_value, new_value, edited_at FROM item_edits
		WHERE item_id = ? ORDER BY edited_at ASC
	`, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to list item edits: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "error", err)
		}
	}()

	var edits []*domain.ItemEdit
	for rows.Next() {
		edit := &domain.ItemEdit{}
		if err := rows.Scan(&edit.ID, &edit.ItemID, &edit.Field, &edit.OldValue, &edit.NewValue, &edit.EditedAt); err != nil {
			return nil, fmt.Errorf("failed to scan item edit: %w", err)
		}
		edits = append(edits, edit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating item edits: %w", err)
	}

	return edits, nil
}
