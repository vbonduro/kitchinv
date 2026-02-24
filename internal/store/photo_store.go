package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vbonduro/kitchinv/internal/domain"
)

type PhotoStore struct {
	db *sql.DB
}

func NewPhotoStore(db *sql.DB) *PhotoStore {
	return &PhotoStore{db: db}
}

func (s *PhotoStore) Create(ctx context.Context, areaID int64, storageKey, mimeType string) (*domain.Photo, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO photos (area_id, storage_key, mime_type) VALUES (?, ?, ?)
	`, areaID, storageKey, mimeType)
	if err != nil {
		return nil, fmt.Errorf("failed to create photo: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return s.GetByID(ctx, id)
}

func (s *PhotoStore) GetByID(ctx context.Context, id int64) (*domain.Photo, error) {
	photo := &domain.Photo{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, area_id, storage_key, mime_type, uploaded_at FROM photos WHERE id = ?
	`, id).Scan(&photo.ID, &photo.AreaID, &photo.StorageKey, &photo.MimeType, &photo.UploadedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get photo: %w", err)
	}

	return photo, nil
}

func (s *PhotoStore) GetLatestByAreaID(ctx context.Context, areaID int64) (*domain.Photo, error) {
	photo := &domain.Photo{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, area_id, storage_key, mime_type, uploaded_at FROM photos
		WHERE area_id = ? ORDER BY uploaded_at DESC LIMIT 1
	`, areaID).Scan(&photo.ID, &photo.AreaID, &photo.StorageKey, &photo.MimeType, &photo.UploadedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get photo: %w", err)
	}

	return photo, nil
}

func (s *PhotoStore) DeleteByArea(ctx context.Context, areaID int64) (*domain.Photo, error) {
	// Get the latest photo first so we can return it for file cleanup.
	photo, err := s.GetLatestByAreaID(ctx, areaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get photo for area: %w", err)
	}
	if photo == nil {
		return nil, nil
	}

	_, err = s.db.ExecContext(ctx, `
		DELETE FROM photos WHERE area_id = ?
	`, areaID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete photos for area: %w", err)
	}

	return photo, nil
}

func (s *PhotoStore) Delete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM photos WHERE id = ?
	`, id)
	if err != nil {
		return fmt.Errorf("failed to delete photo: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("photo not found")
	}

	return nil
}
