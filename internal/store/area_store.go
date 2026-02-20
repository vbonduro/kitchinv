package store

import (
	"context"
	"database/sql"
	"fmt"

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
		INSERT INTO areas (name) VALUES (?)
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
		SELECT id, name, created_at, updated_at FROM areas ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list areas: %w", err)
	}
	defer rows.Close()

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
