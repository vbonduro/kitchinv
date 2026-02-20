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
