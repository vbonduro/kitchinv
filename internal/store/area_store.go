package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/vbonduro/kitchinv/internal/domain"
)

type AreaStore struct {
	db *sql.DB
}

func NewAreaStore(db *sql.DB) *AreaStore {
	return &AreaStore{db: db}
}

func (s *AreaStore) Create(ctx context.Context, name string) (*domain.Area, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO areas (name, sort_order)
		VALUES (?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM areas))
	`, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create area: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return s.GetByID(ctx, id)
}

func (s *AreaStore) GetByID(ctx context.Context, id int64) (*domain.Area, error) {
	area := &domain.Area{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, created_at, updated_at FROM areas WHERE id = ?
	`, id).Scan(&area.ID, &area.Name, &area.CreatedAt, &area.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get area: %w", err)
	}

	return area, nil
}

func (s *AreaStore) List(ctx context.Context) ([]*domain.Area, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, created_at, updated_at FROM areas ORDER BY sort_order ASC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list areas: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "error", err)
		}
	}()

	var areas []*domain.Area
	for rows.Next() {
		area := &domain.Area{}
		if err := rows.Scan(&area.ID, &area.Name, &area.CreatedAt, &area.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan area: %w", err)
		}
		areas = append(areas, area)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating areas: %w", err)
	}

	return areas, nil
}

func (s *AreaStore) Update(ctx context.Context, id int64, name string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE areas SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, name, id)
	if err != nil {
		return fmt.Errorf("failed to update area: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("area not found")
	}

	return nil
}

// UpdateSortOrder sets each area's sort_order to its position in ids (1-based).
// ids must contain all area IDs being reordered; any area not listed is unaffected.
func (s *AreaStore) UpdateSortOrder(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE areas SET sort_order = ? WHERE id = ?`, i+1, id); err != nil {
			return fmt.Errorf("failed to update sort_order for area %d: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit sort order update: %w", err)
	}
	return nil
}

func (s *AreaStore) Delete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM areas WHERE id = ?
	`, id)
	if err != nil {
		return fmt.Errorf("failed to delete area: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("area not found")
	}

	return nil
}
