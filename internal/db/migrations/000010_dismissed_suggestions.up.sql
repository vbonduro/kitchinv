CREATE TABLE dismissed_suggestions (
    item_id   INTEGER NOT NULL,
    old_value TEXT    NOT NULL,
    PRIMARY KEY (item_id, old_value)
);
