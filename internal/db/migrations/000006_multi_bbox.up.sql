-- Replace four scalar bbox columns with a single bboxes TEXT column storing
-- a JSON array of [x1,y1,x2,y2] arrays (e.g. [[0.1,0.2,0.8,0.9]]).
-- NULL means no bbox (user-created items or items without detection coordinates).
-- SQLite table-recreate pattern (same as migration 000005).
CREATE TABLE items_new (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    area_id    INTEGER  NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    photo_id   INTEGER  REFERENCES photos(id) ON DELETE SET NULL,
    name       TEXT     NOT NULL,
    quantity   TEXT     NOT NULL DEFAULT '',
    source     TEXT     NOT NULL DEFAULT 'ai' CHECK(source IN ('ai', 'user')),
    bboxes     TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00'
);

INSERT INTO items_new (id, area_id, photo_id, name, quantity, source, bboxes, created_at, updated_at)
SELECT
    id, area_id, photo_id, name, quantity, source,
    CASE
        WHEN bbox_x1 IS NOT NULL THEN
            json_array(json_array(bbox_x1, bbox_y1, bbox_x2, bbox_y2))
        ELSE NULL
    END,
    created_at, updated_at
FROM items;

DROP TABLE items;
ALTER TABLE items_new RENAME TO items;

CREATE INDEX IF NOT EXISTS idx_items_area_id ON items(area_id);
