package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// OpenForTesting opens an in-memory SQLite database with all migrations applied.
// Use this in tests that need a real database.
func OpenForTesting() (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&mode=rwc&_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open in-memory database: %w", err)
	}
	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	return db, nil
}

// Reset deletes all application data from the database and resets autoincrement
// counters. Intended for use in E2E test mode only; never call this in production.
func Reset(database *sql.DB) error {
	_, err := database.Exec(`DELETE FROM areas`)
	if err != nil {
		return err
	}
	_, err = database.Exec(`DELETE FROM sqlite_sequence WHERE name IN ('areas', 'photos', 'items')`)
	return err
}

func Open(dbPath string) (*sql.DB, error) {
	// cache=shared enables multiple connections to share the same in-memory page
	// cache. mode=rwc creates the file if it does not exist. WAL mode allows
	// concurrent reads alongside a single writer, which matters for a web server.
	// foreign_keys=on enforces referential integrity, which SQLite disables by default.
	dsn := fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		if cerr := db.Close(); cerr != nil {
			return nil, fmt.Errorf("failed to run migrations: %w (also failed to close db: %v)", err, cerr)
		}
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

func runMigrations(db *sql.DB) error {
	// Create migrations table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			dirty BOOLEAN NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of migration files
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Group migrations by version
	type migration struct {
		version int
		name    string
		isUp    bool
	}

	migrations := make(map[int]*migration)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Parse version from filename (e.g., "000001_create_areas.up.sql")
		parts := strings.Split(name, "_")
		if len(parts) < 3 {
			continue
		}

		version := 0
		if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
			slog.Warn("skipping migration file", "file", name, "error", err)
			continue
		}

		// Only up migrations are applied; down migrations are embedded but not
		// executed. Rollback is not currently supported.
		isUp := strings.HasSuffix(name, ".up.sql")
		if !isUp && !strings.HasSuffix(name, ".down.sql") {
			continue
		}

		if _, exists := migrations[version]; !exists {
			migrations[version] = &migration{version: version}
		}

		if isUp {
			migrations[version].isUp = true
		}
		migrations[version].name = name
	}

	// Sort migrations by version
	var versions []int
	for v := range migrations {
		versions = append(versions, v)
	}
	sort.Ints(versions)

	// Apply migrations in order
	for _, version := range versions {
		m := migrations[version]
		if !m.isUp {
			continue
		}

		// Check if already applied
		var applied int
		err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}

		if applied > 0 {
			continue // Already applied
		}

		// Read and execute migration
		data, err := fs.ReadFile(migrationsFS, fmt.Sprintf("migrations/%s", m.name))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", m.name, err)
		}

		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", m.name, err)
		}

		// Record migration
		if _, err := db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}
	}

	return nil
}
