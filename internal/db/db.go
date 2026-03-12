package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// OpenForTesting opens an in-memory SQLite database with all migrations applied.
// Use this in tests that need a real database.
func OpenForTesting() (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&mode=rwc&_journal_mode=WAL&_pragma=foreign_keys(1)")
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
// Tables are deleted in dependency order (items → photos → areas) rather than
// relying on ON DELETE CASCADE, which requires foreign_keys PRAGMA to be active
// on the specific connection executing the DELETE — not guaranteed for all drivers.
func Reset(database *sql.DB) error {
	for _, stmt := range []string{
		`DELETE FROM item_edits`,
		`DELETE FROM items`,
		`DELETE FROM photos`,
		`DELETE FROM areas`,
		`DELETE FROM sqlite_sequence WHERE name IN ('areas', 'photos', 'items', 'item_edits')`,
	} {
		if _, err := database.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func Open(dbPath string) (*sql.DB, error) {
	// cache=shared enables multiple connections to share the same in-memory page
	// cache. mode=rwc creates the file if it does not exist. WAL mode allows
	// concurrent reads alongside a single writer, which matters for a web server.
	// foreign_keys=on enforces referential integrity, which SQLite disables by default.
	dsn := fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool. SQLite WAL allows concurrent reads with a
	// single writer; 10 open connections is sufficient for a home server.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Run migrations
	if err := runMigrations(db); err != nil {
		if cerr := db.Close(); cerr != nil {
			return nil, fmt.Errorf("failed to run migrations: %w (also failed to close db: %v)", err, cerr)
		}
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// execMigration runs a single migration SQL file on a dedicated connection
// with foreign_keys temporarily disabled. This is required for migrations that
// recreate tables (SQLite's only way to drop columns or change constraints),
// where the intermediate DROP TABLE would otherwise violate FK constraints.
func execMigration(db *sql.DB, sqlStr string) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, sqlStr); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	return err
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

		if err := execMigration(db, string(data)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", m.name, err)
		}

		// Record migration
		if _, err := db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}
	}

	return nil
}
