CREATE TABLE IF NOT EXISTS area_snapshots (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    area_id    INTEGER  NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    taken_at   DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    items      TEXT     NOT NULL
);
