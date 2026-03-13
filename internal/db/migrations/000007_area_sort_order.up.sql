-- Add sort_order to areas for user-defined drag-and-drop ordering.
-- Existing areas are assigned initial order alphabetically (matching previous default sort).
ALTER TABLE areas ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

UPDATE areas SET sort_order = (
    SELECT COUNT(*) FROM areas a2 WHERE a2.name < areas.name
) + 1;
