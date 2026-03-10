ALTER TABLE items ADD COLUMN source     TEXT NOT NULL DEFAULT 'ai' CHECK(source IN ('ai', 'user'));
ALTER TABLE items ADD COLUMN updated_at DATETIME NOT NULL DEFAULT (datetime('now'));

CREATE TABLE item_edits (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id   INTEGER  NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    field     TEXT     NOT NULL CHECK(field IN ('name', 'quantity', 'notes')),
    old_value TEXT,
    new_value TEXT,
    edited_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_item_edits_item_id ON item_edits(item_id);
