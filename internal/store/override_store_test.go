package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbonduro/kitchinv/internal/db"
	"github.com/vbonduro/kitchinv/internal/domain"
)

func newOverrideTestDB(t *testing.T) (*OverrideStore, *AreaStore, func()) {
	t.Helper()
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	return NewOverrideStore(d), NewAreaStore(d), func() { _ = d.Close() }
}

func TestOverrideStore_CreateAndGetByID(t *testing.T) {
	s, _, cleanup := newOverrideTestDB(t)
	defer cleanup()
	ctx := context.Background()

	rule, err := s.Create(ctx, domain.OverrideRule{
		MatchPattern:         "Tropicana OJ",
		Replacement:          "Orange Juice",
		MatchExact:           true,
		MatchCaseInsensitive: true,
		Scope:                "global",
		SortOrder:            5,
	})
	require.NoError(t, err)
	assert.NotZero(t, rule.ID)
	assert.Equal(t, "Tropicana OJ", rule.MatchPattern)
	assert.Equal(t, "Orange Juice", rule.Replacement)
	assert.True(t, rule.MatchExact)
	assert.True(t, rule.MatchCaseInsensitive)
	assert.False(t, rule.MatchSubstring)
	assert.Equal(t, "global", rule.Scope)
	assert.Equal(t, 5, rule.SortOrder)
	assert.Empty(t, rule.AreaIDs)

	fetched, err := s.GetByID(ctx, rule.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, rule.ID, fetched.ID)
	assert.Equal(t, "Tropicana OJ", fetched.MatchPattern)
}

func TestOverrideStore_Create_WithAreaIDs(t *testing.T) {
	s, areaStore, cleanup := newOverrideTestDB(t)
	defer cleanup()
	ctx := context.Background()

	area, err := areaStore.Create(ctx, "Fridge")
	require.NoError(t, err)

	rule, err := s.Create(ctx, domain.OverrideRule{
		MatchPattern: "milk",
		Replacement:  "Milk",
		MatchExact:   true,
		Scope:        "area",
		AreaIDs:      []int64{area.ID},
	})
	require.NoError(t, err)
	assert.Equal(t, "area", rule.Scope)
	require.Len(t, rule.AreaIDs, 1)
	assert.Equal(t, area.ID, rule.AreaIDs[0])
}

func TestOverrideStore_List(t *testing.T) {
	s, areaStore, cleanup := newOverrideTestDB(t)
	defer cleanup()
	ctx := context.Background()

	area, err := areaStore.Create(ctx, "Fridge")
	require.NoError(t, err)

	r1, err := s.Create(ctx, domain.OverrideRule{
		MatchPattern: "b", Replacement: "B", MatchExact: true, Scope: "global", SortOrder: 2,
	})
	require.NoError(t, err)
	r2, err := s.Create(ctx, domain.OverrideRule{
		MatchPattern: "a", Replacement: "A", MatchExact: true, Scope: "area",
		AreaIDs: []int64{area.ID}, SortOrder: 1,
	})
	require.NoError(t, err)

	rules, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 2)
	// sorted by sort_order ASC
	assert.Equal(t, r2.ID, rules[0].ID)
	assert.Equal(t, r1.ID, rules[1].ID)
	assert.Len(t, rules[0].AreaIDs, 1)
}

func TestOverrideStore_ListForArea_Priority(t *testing.T) {
	s, areaStore, cleanup := newOverrideTestDB(t)
	defer cleanup()
	ctx := context.Background()

	area1, err := areaStore.Create(ctx, "Fridge")
	require.NoError(t, err)
	area2, err := areaStore.Create(ctx, "Pantry")
	require.NoError(t, err)

	// global rule with order 1
	global, err := s.Create(ctx, domain.OverrideRule{
		MatchPattern: "milk", Replacement: "Milk", MatchExact: true, Scope: "global", SortOrder: 1,
	})
	require.NoError(t, err)

	// area-scoped rule for area1 with same order (should sort before global)
	areaRule, err := s.Create(ctx, domain.OverrideRule{
		MatchPattern: "juice", Replacement: "Juice", MatchExact: true, Scope: "area",
		AreaIDs: []int64{area1.ID}, SortOrder: 1,
	})
	require.NoError(t, err)

	// area2 rule — should NOT appear for area1
	_, err = s.Create(ctx, domain.OverrideRule{
		MatchPattern: "butter", Replacement: "Butter", MatchExact: true, Scope: "area",
		AreaIDs: []int64{area2.ID}, SortOrder: 0,
	})
	require.NoError(t, err)

	rules, err := s.ListForArea(ctx, area1.ID)
	require.NoError(t, err)
	require.Len(t, rules, 2, "should get global + area1-scoped, not area2-scoped")

	// area-scoped beats global at same sort_order
	assert.Equal(t, areaRule.ID, rules[0].ID)
	assert.Equal(t, global.ID, rules[1].ID)
}

func TestOverrideStore_Update(t *testing.T) {
	s, areaStore, cleanup := newOverrideTestDB(t)
	defer cleanup()
	ctx := context.Background()

	area, err := areaStore.Create(ctx, "Fridge")
	require.NoError(t, err)

	rule, err := s.Create(ctx, domain.OverrideRule{
		MatchPattern: "milk", Replacement: "Milk", MatchExact: true, Scope: "global", SortOrder: 0,
	})
	require.NoError(t, err)

	// Update: change to area-scoped with area association
	updated, err := s.Update(ctx, domain.OverrideRule{
		ID:           rule.ID,
		MatchPattern: "whole milk",
		Replacement:  "Whole Milk",
		MatchExact:   true,
		Scope:        "area",
		AreaIDs:      []int64{area.ID},
		SortOrder:    3,
	})
	require.NoError(t, err)
	assert.Equal(t, "whole milk", updated.MatchPattern)
	assert.Equal(t, "area", updated.Scope)
	assert.Equal(t, 3, updated.SortOrder)
	require.Len(t, updated.AreaIDs, 1)
	assert.Equal(t, area.ID, updated.AreaIDs[0])
}

func TestOverrideStore_Delete_CascadesAreas(t *testing.T) {
	s, areaStore, cleanup := newOverrideTestDB(t)
	defer cleanup()
	ctx := context.Background()

	area, err := areaStore.Create(ctx, "Fridge")
	require.NoError(t, err)

	rule, err := s.Create(ctx, domain.OverrideRule{
		MatchPattern: "x", Replacement: "y", MatchExact: true, Scope: "area",
		AreaIDs: []int64{area.ID},
	})
	require.NoError(t, err)

	err = s.Delete(ctx, rule.ID)
	require.NoError(t, err)

	fetched, err := s.GetByID(ctx, rule.ID)
	require.NoError(t, err)
	assert.Nil(t, fetched)

	// Area associations should also be gone.
	ids, err := s.fetchAreaIDs(ctx, rule.ID)
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func TestOverrideStore_CreateFromEdit(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	defer func() { _ = d.Close() }()

	s := NewOverrideStore(d)
	areaStore := NewAreaStore(d)
	ctx := context.Background()

	area, err := areaStore.Create(ctx, "Fridge")
	require.NoError(t, err)

	// Creates a rule at the top.
	err = s.CreateFromEdit(ctx, area.ID, "OJ", "Orange Juice")
	require.NoError(t, err)

	rules, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, "OJ", rules[0].MatchPattern)
	assert.Equal(t, "Orange Juice", rules[0].Replacement)
	assert.True(t, rules[0].MatchExact)
	assert.True(t, rules[0].MatchCaseInsensitive)
	assert.False(t, rules[0].MatchSubstring)
	assert.Equal(t, "area", rules[0].Scope)
	assert.Equal(t, []int64{area.ID}, rules[0].AreaIDs)

	// Second call with same pattern+area is idempotent.
	err = s.CreateFromEdit(ctx, area.ID, "OJ", "OJ Updated")
	require.NoError(t, err)
	rules, err = s.List(ctx)
	require.NoError(t, err)
	assert.Len(t, rules, 1, "duplicate rule should not be created")

	// New rule sorts above the first.
	err = s.CreateFromEdit(ctx, area.ID, "Milk", "Whole Milk")
	require.NoError(t, err)
	rules, err = s.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, "Milk", rules[0].MatchPattern, "newer rule should sort first")
}

func TestOverrideStore_DeleteOrphanedAreaRules(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	defer func() { _ = d.Close() }()

	s := NewOverrideStore(d)
	areaStore := NewAreaStore(d)
	ctx := context.Background()

	area, err := areaStore.Create(ctx, "Pantry")
	require.NoError(t, err)

	err = s.CreateFromEdit(ctx, area.ID, "Milk", "Whole Milk")
	require.NoError(t, err)

	rules, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)

	// Delete the area (cascades the override_rule_areas row).
	err = areaStore.Delete(ctx, area.ID)
	require.NoError(t, err)

	// Rule is now orphaned — clean it up.
	err = s.DeleteOrphanedAreaRules(ctx)
	require.NoError(t, err)

	rules, err = s.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, rules)
}
