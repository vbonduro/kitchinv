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
	assert.Equal(t, "Garage Fridge", areas[0].Name)
	assert.Equal(t, "Pantry", areas[1].Name)
}

func TestAreaStoreList_Empty(t *testing.T) {
	d := openTestDB(t)
	store := NewAreaStore(d)
	ctx := context.Background()

	areas, err := store.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, areas)
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
