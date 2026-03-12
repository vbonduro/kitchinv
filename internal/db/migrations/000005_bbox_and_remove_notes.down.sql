-- Restore items table with notes column and without bbox columns.
CREATE TABLE items_old (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    area_id    INTEGER  NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    photo_id   INTEGER  REFERENCES photos(id) ON DELETE SET NULL,
    name       TEXT     NOT NULL,
    quantity   TEXT     NOT NULL DEFAULT '',
    notes      TEXT     NOT NULL DEFAULT '',
    source     TEXT     NOT NULL DEFAULT 'ai' CHECK(source IN ('ai', 'user')),
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00'
);

INSERT INTO items_old (id, area_id, photo_id, name, quantity, notes, source, created_at, updated_at)
SELECT id, area_id, photo_id, name, quantity, '', source, created_at, updated_at
FROM items;

DROP TABLE items;
ALTER TABLE items_old RENAME TO items;

CREATE INDEX IF NOT EXISTS idx_items_area_id ON items(area_id);

-- Restore item_edits with notes field allowed.
CREATE TABLE item_edits_old (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id   INTEGER  NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    field     TEXT     NOT NULL CHECK(field IN ('name', 'quantity', 'notes')),
    old_value TEXT,
    new_value TEXT,
    edited_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO item_edits_old (id, item_id, field, old_value, new_value, edited_at)
SELECT id, item_id, field, old_value, new_value, edited_at FROM item_edits;

DROP TABLE item_edits;
ALTER TABLE item_edits_old RENAME TO item_edits;

CREATE INDEX IF NOT EXISTS idx_item_edits_item_id ON item_edits(item_id);
