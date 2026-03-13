-- Restore four scalar bbox columns from the JSON bboxes column.
-- Only the first bbox in the array is preserved (data loss for multi-bbox items).
CREATE TABLE items_old (
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

INSERT INTO items_old (id, area_id, photo_id, name, quantity, source, bbox_x1, bbox_y1, bbox_x2, bbox_y2, created_at, updated_at)
SELECT
    id, area_id, photo_id, name, quantity, source,
    CASE WHEN bboxes IS NOT NULL THEN json_extract(bboxes, '$[0][0]') ELSE NULL END,
    CASE WHEN bboxes IS NOT NULL THEN json_extract(bboxes, '$[0][1]') ELSE NULL END,
    CASE WHEN bboxes IS NOT NULL THEN json_extract(bboxes, '$[0][2]') ELSE NULL END,
    CASE WHEN bboxes IS NOT NULL THEN json_extract(bboxes, '$[0][3]') ELSE NULL END,
    created_at, updated_at
FROM items;

DROP TABLE items;
ALTER TABLE items_old RENAME TO items;

CREATE INDEX IF NOT EXISTS idx_items_area_id ON items(area_id);
