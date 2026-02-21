package db

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenInMemory(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&mode=rwc&_journal_mode=WAL&_foreign_keys=on")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	err = db.Ping()
	assert.NoError(t, err)
}

func TestMigrationsApply(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&mode=rwc&_journal_mode=WAL&_foreign_keys=on")
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
}
