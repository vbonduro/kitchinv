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

// CreateFromEdit creates an area-scoped, case-insensitive, exact-match override rule
// automatically when a user renames an item. The rule is inserted at the top of the
// list (sort_order = MIN(sort_order) - 1). If a rule with the same pattern already
// exists for the same area it is left unchanged and no error is returned.
func (s *OverrideStore) CreateFromEdit(ctx context.Context, areaID int64, oldName, newName string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Check if an identical rule already covers this area.
	var count int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM override_rules r
		JOIN override_rule_areas ora ON ora.rule_id = r.id
		WHERE r.match_pattern = ? AND ora.area_id = ?
	`, oldName, areaID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing rule: %w", err)
	}
	if count > 0 {
		return nil // already covered
	}

	// Place at the top: MIN(sort_order) - 1, or 0 if no rules yet.
	var minOrder int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MIN(sort_order), 1) FROM override_rules`).Scan(&minOrder); err != nil {
		return fmt.Errorf("failed to get min sort_order: %w", err)
	}
	sortOrder := minOrder - 1

	result, err := tx.ExecContext(ctx, `
		INSERT INTO override_rules
			(match_pattern, replacement, match_exact, match_case_insensitive, match_substring, scope, sort_order)
		VALUES (?, ?, 1, 1, 0, 'area', ?)
	`, oldName, newName, sortOrder)
	if err != nil {
		return fmt.Errorf("failed to insert override rule from edit: %w", err)
	}

	ruleID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	if err := insertRuleAreas(ctx, tx, ruleID, []int64{areaID}); err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteOrphanedAreaRules removes area-scoped rules that have no remaining area
// associations (e.g. after their only associated area was deleted).
func (s *OverrideStore) DeleteOrphanedAreaRules(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM override_rules
		WHERE scope = 'area'
		  AND id NOT IN (SELECT DISTINCT rule_id FROM override_rule_areas)
	`)
	if err != nil {
		return fmt.Errorf("failed to delete orphaned area rules: %w", err)
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
