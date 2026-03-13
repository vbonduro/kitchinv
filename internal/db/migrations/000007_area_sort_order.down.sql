-- SQLite does not support DROP COLUMN in older versions; recreate the table.
CREATE TABLE areas_new (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO areas_new (id, name, created_at, updated_at)
SELECT id, name, created_at, updated_at FROM areas;

DROP TABLE areas;
ALTER TABLE areas_new RENAME TO areas;
