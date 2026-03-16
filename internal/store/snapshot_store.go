package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vbonduro/kitchinv/internal/domain"
)

type SnapshotStore struct {
	db *sql.DB
}

func NewSnapshotStore(db *sql.DB) *SnapshotStore {
	return &SnapshotStore{db: db}
}

func (s *SnapshotStore) Create(ctx context.Context, areaID int64, items []domain.SnapshotItem) (*domain.Snapshot, error) {
	data, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("failed to encode snapshot items: %w", err)
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO area_snapshots (area_id, items) VALUES (?, ?)`,
		areaID, string(data),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert snapshot: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot id: %w", err)
	}

	var takenAt time.Time
	if err := s.db.QueryRowContext(ctx, `SELECT taken_at FROM area_snapshots WHERE id = ?`, id).Scan(&takenAt); err != nil {
		return nil, fmt.Errorf("failed to read snapshot taken_at: %w", err)
	}

	return &domain.Snapshot{
		ID:      id,
		AreaID:  areaID,
		TakenAt: takenAt,
		Items:   items,
	}, nil
}

func (s *SnapshotStore) ListByAreaID(ctx context.Context, areaID int64) ([]*domain.Snapshot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, area_id, taken_at, items FROM area_snapshots WHERE area_id = ? ORDER BY taken_at DESC`,
		areaID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	snapshots := make([]*domain.Snapshot, 0)
	for rows.Next() {
		var snap domain.Snapshot
		var itemsJSON string
		if err := rows.Scan(&snap.ID, &snap.AreaID, &snap.TakenAt, &itemsJSON); err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}
		if err := json.Unmarshal([]byte(itemsJSON), &snap.Items); err != nil {
			return nil, fmt.Errorf("failed to decode snapshot items: %w", err)
		}
		snapshots = append(snapshots, &snap)
	}
	return snapshots, rows.Err()
}
