package db

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenInMemory(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&mode=rwc&_journal_mode=WAL&_pragma=foreign_keys(1)")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	err = db.Ping()
	assert.NoError(t, err)
}

func TestMigrationsApply(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&mode=rwc&_journal_mode=WAL&_pragma=foreign_keys(1)")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	err = runMigrations(db)
	assert.NoError(t, err)

	// Verify tables exist
	var tableName string

	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='areas'").Scan(&tableName)
	assert.NoError(t, err)
	assert.Equal(t, "areas", tableName)

	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='photos'").Scan(&tableName)
	assert.NoError(t, err)
	assert.Equal(t, "photos", tableName)

	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='items'").Scan(&tableName)
	assert.NoError(t, err)
	assert.Equal(t, "items", tableName)

	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='item_edits'").Scan(&tableName)
	assert.NoError(t, err)
	assert.Equal(t, "item_edits", tableName)
}

// TestMigrationsSchema verifies that all expected columns exist with the right
// types after migrations run. This catches SQL errors like non-constant defaults
// in ALTER TABLE ADD COLUMN that would fail on existing databases.
func TestMigrationsSchema(t *testing.T) {
	db, err := OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	type col struct{ name, typ string }
	checkColumns := func(table string, want []col) {
		t.Helper()
		rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
		require.NoError(t, err, "PRAGMA table_info(%s)", table)
		defer func() { assert.NoError(t, rows.Close()) }()

		got := map[string]string{}
		for rows.Next() {
			var cid int
			var name, typ string
			var notnull int
			var dflt sql.NullString
			var pk int
			require.NoError(t, rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk))
			got[name] = typ
		}
		for _, w := range want {
			assert.Equal(t, w.typ, got[w.name], "table %s: column %s type", table, w.name)
		}
	}

	checkColumns("items", []col{
		{"id", "INTEGER"},
		{"area_id", "INTEGER"},
		{"photo_id", "INTEGER"},
		{"name", "TEXT"},
		{"quantity", "TEXT"},
		{"source", "TEXT"},
		{"bboxes", "TEXT"},
		{"created_at", "DATETIME"},
		{"updated_at", "DATETIME"},
	})

	checkColumns("item_edits", []col{
		{"id", "INTEGER"},
		{"item_id", "INTEGER"},
		{"field", "TEXT"},
		{"old_value", "TEXT"},
		{"new_value", "TEXT"},
		{"edited_at", "DATETIME"},
	})
}

// TestMigrationsIdempotent verifies that running migrations twice does not
// error — each migration must only be applied once.
func TestMigrationsIdempotent(t *testing.T) {
	db, err := OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	err = runMigrations(db)
	assert.NoError(t, err, "running migrations a second time should be a no-op")
}
