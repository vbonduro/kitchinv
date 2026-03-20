CREATE TABLE override_rules (
    id                     INTEGER  PRIMARY KEY AUTOINCREMENT,
    match_pattern          TEXT     NOT NULL,
    replacement            TEXT     NOT NULL DEFAULT '',
    match_exact            BOOLEAN  NOT NULL DEFAULT 0,
    match_case_insensitive BOOLEAN  NOT NULL DEFAULT 0,
    match_substring        BOOLEAN  NOT NULL DEFAULT 0,
    scope                  TEXT     NOT NULL DEFAULT 'global'
                                    CHECK(scope IN ('global', 'area')),
    sort_order             INTEGER  NOT NULL DEFAULT 0,
    created_at             DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE override_rule_areas (
    rule_id  INTEGER NOT NULL REFERENCES override_rules(id) ON DELETE CASCADE,
    area_id  INTEGER NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    PRIMARY KEY (rule_id, area_id)
);

CREATE INDEX idx_override_rules_sort      ON override_rules(sort_order ASC);
CREATE INDEX idx_override_rule_areas_rule ON override_rule_areas(rule_id);
CREATE INDEX idx_override_rule_areas_area ON override_rule_areas(area_id);
