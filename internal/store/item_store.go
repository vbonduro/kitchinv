package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/vbonduro/kitchinv/internal/domain"
)

type ItemStore struct {
	db *sql.DB
}

func NewItemStore(db *sql.DB) *ItemStore {
	return &ItemStore{db: db}
}

func encodeBBoxes(bboxes [][]float64) sql.NullString {
	if len(bboxes) == 0 {
		return sql.NullString{}
	}
	b, err := json.Marshal(bboxes)
	if err != nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(b), Valid: true}
}

func decodeBBoxes(ns sql.NullString) [][]float64 {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	var bboxes [][]float64
	if err := json.Unmarshal([]byte(ns.String), &bboxes); err != nil {
		return nil
	}
	return bboxes
}

func (s *ItemStore) Create(ctx context.Context, areaID int64, photoID *int64, name, quantity, source string, bboxes [][]float64) (*domain.Item, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO items (area_id, photo_id, name, quantity, source, bboxes)
		VALUES (?, ?, ?, ?, ?, ?)
	`, areaID, photoID, name, quantity, source, encodeBBoxes(bboxes))
	if err != nil {
		return nil, fmt.Errorf("failed to create item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return s.GetByID(ctx, id)
}

func (s *ItemStore) GetByID(ctx context.Context, id int64) (*domain.Item, error) {
	item := &domain.Item{}
	var bboxesRaw sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, area_id, photo_id, name, quantity, source, bboxes, created_at, updated_at
		FROM items WHERE id = ?
	`, id).Scan(
		&item.ID, &item.AreaID, &item.PhotoID,
		&item.Name, &item.Quantity, &item.Source,
		&bboxesRaw,
		&item.CreatedAt, &item.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	item.BBoxes = decodeBBoxes(bboxesRaw)
	return item, nil
}

func (s *ItemStore) ListByAreaID(ctx context.Context, areaID int64) ([]*domain.Item, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, area_id, photo_id, name, quantity, source, bboxes, created_at, updated_at
		FROM items WHERE area_id = ? ORDER BY name ASC
	`, areaID)
	if err != nil {
		return nil, fmt.Errorf("failed to list items: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "error", err)
		}
	}()

	var items []*domain.Item
	for rows.Next() {
		item := &domain.Item{}
		var bboxesRaw sql.NullString
		if err := rows.Scan(
			&item.ID, &item.AreaID, &item.PhotoID,
			&item.Name, &item.Quantity, &item.Source,
			&bboxesRaw,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		item.BBoxes = decodeBBoxes(bboxesRaw)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating items: %w", err)
	}

	return items, nil
}

func (s *ItemStore) Search(ctx context.Context, query string) ([]*domain.Item, error) {
	pattern := "%" + strings.ToLower(query) + "%"

	rows, err := s.db.QueryContext(ctx, `
		SELECT i.id, i.area_id, i.photo_id, i.name, i.quantity, i.source,
		       i.bboxes, i.created_at, i.updated_at
		FROM items i
		INNER JOIN areas a ON i.area_id = a.id
		WHERE LOWER(i.name) LIKE ?
		ORDER BY i.name ASC
	`, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search items: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "error", err)
		}
	}()

	var items []*domain.Item
	for rows.Next() {
		item := &domain.Item{}
		var bboxesRaw sql.NullString
		if err := rows.Scan(
			&item.ID, &item.AreaID, &item.PhotoID,
			&item.Name, &item.Quantity, &item.Source,
			&bboxesRaw,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		item.BBoxes = decodeBBoxes(bboxesRaw)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating items: %w", err)
	}

	return items, nil
}

func (s *ItemStore) Update(ctx context.Context, id int64, name, quantity string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE items SET name = ?, quantity = ?, updated_at = datetime('now') WHERE id = ?
	`, name, quantity, id)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("item not found")
	}

	return nil
}

func (s *ItemStore) Delete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM items WHERE id = ?
	`, id)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("item not found")
	}

	return nil
}

func (s *ItemStore) DeleteByAreaID(ctx context.Context, areaID int64) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM items WHERE area_id = ?
	`, areaID)
	if err != nil {
		return fmt.Errorf("failed to delete items: %w", err)
	}

	return nil
}
