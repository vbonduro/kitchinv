package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/vbonduro/kitchinv/internal/domain"
)

// OverrideStore persists override rules and their area associations.
type OverrideStore struct {
	db *sql.DB
}

// NewOverrideStore creates a new OverrideStore backed by db.
func NewOverrideStore(db *sql.DB) *OverrideStore {
	return &OverrideStore{db: db}
}

// Create inserts a new override rule and its area associations in a transaction.
func (s *OverrideStore) Create(ctx context.Context, r domain.OverrideRule) (*domain.OverrideRule, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO override_rules
			(match_pattern, replacement, match_exact, match_case_insensitive, match_substring, scope, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, r.MatchPattern, r.Replacement, r.MatchExact, r.MatchCaseInsensitive, r.MatchSubstring, r.Scope, r.SortOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to insert override rule: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	if err := insertRuleAreas(ctx, tx, id, r.AreaIDs); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return s.GetByID(ctx, id)
}

// GetByID fetches a single override rule by ID.
func (s *OverrideStore) GetByID(ctx context.Context, id int64) (*domain.OverrideRule, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, match_pattern, replacement, match_exact, match_case_insensitive,
		       match_substring, scope, sort_order, created_at
		FROM override_rules WHERE id = ?
	`, id)

	r := &domain.OverrideRule{}
	if err := row.Scan(&r.ID, &r.MatchPattern, &r.Replacement, &r.MatchExact, &r.MatchCaseInsensitive,
		&r.MatchSubstring, &r.Scope, &r.SortOrder, &r.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan override rule: %w", err)
	}

	areaIDs, err := s.fetchAreaIDs(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	r.AreaIDs = areaIDs
	return r, nil
}

// List returns all override rules sorted by sort_order ASC, created_at ASC,
// with AreaIDs populated via GROUP_CONCAT to avoid N+1 queries.
func (s *OverrideStore) List(ctx context.Context) ([]*domain.OverrideRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.match_pattern, r.replacement,
		       r.match_exact, r.match_case_insensitive, r.match_substring,
		       r.scope, r.sort_order, r.created_at,
		       GROUP_CONCAT(ora.area_id) AS area_ids
		FROM override_rules r
		LEFT JOIN override_rule_areas ora ON ora.rule_id = r.id
		GROUP BY r.id
		ORDER BY r.sort_order ASC, r.created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list override rules: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "error", err)
		}
	}()

	var rules []*domain.OverrideRule
	for rows.Next() {
		r := &domain.OverrideRule{}
		var areaIDsStr sql.NullString
		if err := rows.Scan(&r.ID, &r.MatchPattern, &r.Replacement,
			&r.MatchExact, &r.MatchCaseInsensitive, &r.MatchSubstring,
			&r.Scope, &r.SortOrder, &r.CreatedAt, &areaIDsStr); err != nil {
			return nil, fmt.Errorf("failed to scan override rule: %w", err)
		}
		r.AreaIDs = parseAreaIDs(areaIDsStr)
		rules = append(rules, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating override rules: %w", err)
	}
	return rules, nil
}

// ListForArea returns global rules plus rules scoped to areaID,
// ordered by sort_order ASC then area-scoped before global within the same order.
func (s *OverrideStore) ListForArea(ctx context.Context, areaID int64) ([]*domain.OverrideRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.match_pattern, r.replacement,
		       r.match_exact, r.match_case_insensitive, r.match_substring,
		       r.scope, r.sort_order, r.created_at
		FROM override_rules r
		WHERE r.scope = 'global'
		   OR EXISTS (SELECT 1 FROM override_rule_areas ora WHERE ora.rule_id = r.id AND ora.area_id = ?)
		ORDER BY r.sort_order ASC,
		         CASE r.scope WHEN 'area' THEN 0 ELSE 1 END ASC
	`, areaID)
	if err != nil {
		return nil, fmt.Errorf("failed to list override rules for area: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "error", err)
		}
	}()

	var rules []*domain.OverrideRule
	for rows.Next() {
		r := &domain.OverrideRule{}
		if err := rows.Scan(&r.ID, &r.MatchPattern, &r.Replacement,
			&r.MatchExact, &r.MatchCaseInsensitive, &r.MatchSubstring,
			&r.Scope, &r.SortOrder, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan override rule: %w", err)
		}
		rules = append(rules, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating override rules: %w", err)
	}

	// Populate AreaIDs for each rule.
	for _, r := range rules {
		areaIDs, err := s.fetchAreaIDs(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		r.AreaIDs = areaIDs
	}

	return rules, nil
}

// Update replaces the rule fields and re-syncs its area associations.
func (s *OverrideStore) Update(ctx context.Context, r domain.OverrideRule) (*domain.OverrideRule, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		UPDATE override_rules
		SET match_pattern = ?, replacement = ?, match_exact = ?,
		    match_case_insensitive = ?, match_substring = ?,
		    scope = ?, sort_order = ?
		WHERE id = ?
	`, r.MatchPattern, r.Replacement, r.MatchExact, r.MatchCaseInsensitive,
		r.MatchSubstring, r.Scope, r.SortOrder, r.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update override rule: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM override_rule_areas WHERE rule_id = ?`, r.ID); err != nil {
		return nil, fmt.Errorf("failed to delete area associations: %w", err)
	}

	if err := insertRuleAreas(ctx, tx, r.ID, r.AreaIDs); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return s.GetByID(ctx, r.ID)
}

// ReorderSortOrder sets each rule's sort_order to its 1-based position in ids.
func (s *OverrideStore) ReorderSortOrder(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE override_rules SET sort_order = ? WHERE id = ?`, i+1, id); err != nil {
			return fmt.Errorf("failed to update sort_order for rule %d: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit sort order update: %w", err)
	}
	return nil
}

// Delete removes an override rule (cascade deletes area associations).
func (s *OverrideStore) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM override_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete override rule: %w", err)
	}
	return nil
}

// ListEditSuggestions returns the 50 most recent name renames for items that still exist.
func (s *OverrideStore) ListEditSuggestions(ctx context.Context) ([]*domain.EditSuggestion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ie.item_id, ie.old_value, ie.new_value,
		       i.area_id, a.name AS area_name, ie.edited_at
		FROM item_edits ie
		LEFT JOIN items i ON ie.item_id = i.id
		LEFT JOIN areas a ON i.area_id = a.id
		WHERE ie.field = 'name' AND i.id IS NOT NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM dismissed_suggestions ds
		      WHERE ds.item_id = ie.item_id AND ds.old_value = ie.old_value
		  )
		ORDER BY ie.edited_at DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list edit suggestions: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "error", err)
		}
	}()

	var suggestions []*domain.EditSuggestion
	for rows.Next() {
		s := &domain.EditSuggestion{}
		if err := rows.Scan(&s.ItemID, &s.OldName, &s.NewName, &s.AreaID, &s.AreaName, &s.EditedAt); err != nil {
			return nil, fmt.Errorf("failed to scan edit suggestion: %w", err)
		}
		suggestions = append(suggestions, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating edit suggestions: %w", err)
	}
	return suggestions, nil
}

// DismissSuggestion marks a suggestion as dismissed so it won't appear again.
func (s *OverrideStore) DismissSuggestion(ctx context.Context, itemID int64, oldName string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO dismissed_suggestions (item_id, old_value) VALUES (?, ?)`,
		itemID, oldName,
	)
	if err != nil {
		return fmt.Errorf("failed to dismiss suggestion: %w", err)
	}
	return nil
}

// fetchAreaIDs returns the area IDs associated with a rule.
func (s *OverrideStore) fetchAreaIDs(ctx context.Context, ruleID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT area_id FROM override_rule_areas WHERE rule_id = ?`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch area ids: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "error", err)
		}
	}()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan area id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// insertRuleAreas inserts area associations for a rule within a transaction.
func insertRuleAreas(ctx context.Context, tx *sql.Tx, ruleID int64, areaIDs []int64) error {
	for _, areaID := range areaIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO override_rule_areas (rule_id, area_id) VALUES (?, ?)`,
			ruleID, areaID); err != nil {
			return fmt.Errorf("failed to insert rule area association: %w", err)
		}
	}
	return nil
}

// parseAreaIDs splits a comma-separated string of area IDs from GROUP_CONCAT.
func parseAreaIDs(s sql.NullString) []int64 {
	if !s.Valid || s.String == "" {
		return nil
	}
	parts := strings.Split(s.String, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
