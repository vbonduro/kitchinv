CREATE TABLE photos (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    area_id     INTEGER NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    storage_key TEXT    NOT NULL,
    mime_type   TEXT    NOT NULL DEFAULT 'image/jpeg',
    uploaded_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_photos_area_id ON photos(area_id);
