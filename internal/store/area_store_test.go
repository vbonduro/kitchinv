package store

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbonduro/kitchinv/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })
	return d
}

func TestAreaStoreCreate(t *testing.T) {
	d := openTestDB(t)
	store := NewAreaStore(d)
	ctx := context.Background()

	area, err := store.Create(ctx, "Upstairs Fridge")
	require.NoError(t, err)
	assert.NotZero(t, area.ID)
	assert.Equal(t, "Upstairs Fridge", area.Name)
}

func TestAreaStoreCreate_DuplicateName(t *testing.T) {
	d := openTestDB(t)
	store := NewAreaStore(d)
	ctx := context.Background()

	_, err := store.Create(ctx, "Pantry")
	require.NoError(t, err)

	_, err = store.Create(ctx, "Pantry")
	assert.Error(t, err)
}

func TestAreaStoreGetByID(t *testing.T) {
	d := openTestDB(t)
	store := NewAreaStore(d)
	ctx := context.Background()

	created, err := store.Create(ctx, "Downstairs Freezer")
	require.NoError(t, err)

	retrieved, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Name, retrieved.Name)
}

func TestAreaStoreGetByID_NotFound(t *testing.T) {
	d := openTestDB(t)
	store := NewAreaStore(d)
	ctx := context.Background()

	retrieved, err := store.GetByID(ctx, 99999)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestAreaStoreList(t *testing.T) {
	d := openTestDB(t)
	store := NewAreaStore(d)
	ctx := context.Background()

	_, err := store.Create(ctx, "Pantry")
	require.NoError(t, err)
	_, err = store.Create(ctx, "Garage Fridge")
	require.NoError(t, err)

	areas, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, areas, 2)
	// List returns areas in insertion (sort_order) order, not alphabetically.
	assert.Equal(t, "Pantry", areas[0].Name)
	assert.Equal(t, "Garage Fridge", areas[1].Name)
}

func TestAreaStoreList_Empty(t *testing.T) {
	d := openTestDB(t)
	store := NewAreaStore(d)
	ctx := context.Background()

	areas, err := store.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, areas)
}

func TestAreaStoreUpdate(t *testing.T) {
	d := openTestDB(t)
	store := NewAreaStore(d)
	ctx := context.Background()

	created, err := store.Create(ctx, "Old Name")
	require.NoError(t, err)

	err = store.Update(ctx, created.ID, "New Name")
	require.NoError(t, err)

	retrieved, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", retrieved.Name)
}

func TestAreaStoreUpdate_NotFound(t *testing.T) {
	d := openTestDB(t)
	store := NewAreaStore(d)
	ctx := context.Background()

	err := store.Update(ctx, 99999, "New Name")
	assert.Error(t, err)
}

func TestAreaStoreDelete(t *testing.T) {
	d := openTestDB(t)
	store := NewAreaStore(d)
	ctx := context.Background()

	created, err := store.Create(ctx, "Temp Area")
	require.NoError(t, err)

	err = store.Delete(ctx, created.ID)
	require.NoError(t, err)

	retrieved, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestAreaStoreDelete_NotFound(t *testing.T) {
	d := openTestDB(t)
	store := NewAreaStore(d)
	ctx := context.Background()

	err := store.Delete(ctx, 99999)
	assert.Error(t, err)
}

func TestAreaStoreUpdateSortOrder(t *testing.T) {
	d := openTestDB(t)
	s := NewAreaStore(d)
	ctx := context.Background()

	a1, err := s.Create(ctx, "Alpha")
	require.NoError(t, err)
	a2, err := s.Create(ctx, "Beta")
	require.NoError(t, err)
	a3, err := s.Create(ctx, "Gamma")
	require.NoError(t, err)

	// Reverse the order: Gamma, Alpha, Beta
	err = s.UpdateSortOrder(ctx, []int64{a3.ID, a1.ID, a2.ID})
	require.NoError(t, err)

	areas, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, areas, 3)
	assert.Equal(t, "Gamma", areas[0].Name)
	assert.Equal(t, "Alpha", areas[1].Name)
	assert.Equal(t, "Beta", areas[2].Name)
}

func TestAreaStoreUpdateSortOrder_Empty(t *testing.T) {
	d := openTestDB(t)
	s := NewAreaStore(d)
	ctx := context.Background()

	err := s.UpdateSortOrder(ctx, []int64{})
	require.NoError(t, err)
}
