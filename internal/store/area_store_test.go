package store

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	d, err := sql.Open("sqlite", "file::memory:?cache=shared&mode=rwc&_journal_mode=WAL&_foreign_keys=on")
	require.NoError(t, err)

	// Create tables manually for test
	_, err = d.Exec(`
		CREATE TABLE areas (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT    NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		);

		CREATE TABLE photos (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			area_id     INTEGER NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
			storage_key TEXT    NOT NULL,
			mime_type   TEXT    NOT NULL DEFAULT 'image/jpeg',
			uploaded_at DATETIME NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX idx_photos_area_id ON photos(area_id);

		CREATE TABLE items (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			area_id    INTEGER NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
			photo_id   INTEGER REFERENCES photos(id) ON DELETE SET NULL,
			name       TEXT    NOT NULL,
			quantity   TEXT,
			notes      TEXT,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX idx_items_area_id ON items(area_id);
		CREATE INDEX idx_items_name    ON items(name COLLATE NOCASE);
	`)
	require.NoError(t, err)

	return d
}

func TestAreaStoreCreate(t *testing.T) {
	d := openTestDB(t)
	t.Cleanup(func() { _ = d.Close() })

	store := NewAreaStore(d)
	ctx := context.Background()

	area, err := store.Create(ctx, "Upstairs Fridge")
	require.NoError(t, err)
	assert.NotZero(t, area.ID)
	assert.Equal(t, "Upstairs Fridge", area.Name)
}

func TestAreaStoreGetByID(t *testing.T) {
	d := openTestDB(t)
	t.Cleanup(func() { _ = d.Close() })

	store := NewAreaStore(d)
	ctx := context.Background()

	created, err := store.Create(ctx, "Downstairs Freezer")
	require.NoError(t, err)

	retrieved, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Name, retrieved.Name)
}

func TestAreaStoreList(t *testing.T) {
	d := openTestDB(t)
	t.Cleanup(func() { _ = d.Close() })

	store := NewAreaStore(d)
	ctx := context.Background()

	_, err := store.Create(ctx, "Pantry")
	require.NoError(t, err)
	_, err = store.Create(ctx, "Garage Fridge")
	require.NoError(t, err)

	areas, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, areas, 2)
	assert.Equal(t, "Garage Fridge", areas[0].Name)
	assert.Equal(t, "Pantry", areas[1].Name)
}

func TestAreaStoreDelete(t *testing.T) {
	d := openTestDB(t)
	t.Cleanup(func() { _ = d.Close() })

	store := NewAreaStore(d)
	ctx := context.Background()

	created, err := store.Create(ctx, "Temp Area")
	require.NoError(t, err)

	err = store.Delete(ctx, created.ID)
	require.NoError(t, err)

	retrieved, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}
