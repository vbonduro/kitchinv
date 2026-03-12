ALTER TABLE items ADD COLUMN bbox_x1 REAL;
ALTER TABLE items ADD COLUMN bbox_y1 REAL;
ALTER TABLE items ADD COLUMN bbox_x2 REAL;
ALTER TABLE items ADD COLUMN bbox_y2 REAL;

-- SQLite does not support DROP COLUMN before 3.35.0; we recreate the table
-- to drop the notes column.
CREATE TABLE items_new (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    area_id    INTEGER  NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    photo_id   INTEGER  REFERENCES photos(id) ON DELETE SET NULL,
    name       TEXT     NOT NULL,
    quantity   TEXT     NOT NULL DEFAULT '',
    source     TEXT     NOT NULL DEFAULT 'ai' CHECK(source IN ('ai', 'user')),
    bbox_x1    REAL,
    bbox_y1    REAL,
    bbox_x2    REAL,
    bbox_y2    REAL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00'
);

INSERT INTO items_new (id, area_id, photo_id, name, quantity, source, bbox_x1, bbox_y1, bbox_x2, bbox_y2, created_at, updated_at)
SELECT id, area_id, photo_id, name, quantity, source, bbox_x1, bbox_y1, bbox_x2, bbox_y2, created_at, updated_at
FROM items;

DROP TABLE items;
ALTER TABLE items_new RENAME TO items;

CREATE INDEX IF NOT EXISTS idx_items_area_id ON items(area_id);

-- Update item_edits field constraint to remove 'notes'.
-- SQLite does not support ALTER COLUMN so we recreate item_edits too.
CREATE TABLE item_edits_new (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id   INTEGER  NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    field     TEXT     NOT NULL CHECK(field IN ('name', 'quantity')),
    old_value TEXT,
    new_value TEXT,
    edited_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO item_edits_new (id, item_id, field, old_value, new_value, edited_at)
SELECT id, item_id, field, old_value, new_value, edited_at
FROM item_edits
WHERE field IN ('name', 'quantity');

DROP TABLE item_edits;
ALTER TABLE item_edits_new RENAME TO item_edits;

CREATE INDEX IF NOT EXISTS idx_item_edits_item_id ON item_edits(item_id);
