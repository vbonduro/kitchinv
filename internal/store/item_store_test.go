package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestItemStoreCreate(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Fridge")
	require.NoError(t, err)

	item, err := items.Create(ctx, area.ID, nil, "Milk", "1 liter", "opened")
	require.NoError(t, err)
	assert.NotZero(t, item.ID)
	assert.Equal(t, area.ID, item.AreaID)
	assert.Equal(t, "Milk", item.Name)
	assert.Equal(t, "1 liter", item.Quantity)
	assert.Equal(t, "opened", item.Notes)
	assert.Nil(t, item.PhotoID)
}

func TestItemStoreListByAreaID(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Pantry")
	require.NoError(t, err)

	_, err = items.Create(ctx, area.ID, nil, "Rice", "2 kg", "")
	require.NoError(t, err)
	_, err = items.Create(ctx, area.ID, nil, "Pasta", "500 g", "")
	require.NoError(t, err)

	list, err := items.ListByAreaID(ctx, area.ID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	// Results should be alphabetical
	assert.Equal(t, "Pasta", list[0].Name)
	assert.Equal(t, "Rice", list[1].Name)
}

func TestItemStoreListByAreaID_Empty(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Empty Shelf")
	require.NoError(t, err)

	list, err := items.ListByAreaID(ctx, area.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestItemStoreSearch(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Kitchen")
	require.NoError(t, err)

	_, err = items.Create(ctx, area.ID, nil, "Whole Milk", "1 liter", "")
	require.NoError(t, err)
	_, err = items.Create(ctx, area.ID, nil, "Oat Milk", "1 liter", "")
	require.NoError(t, err)
	_, err = items.Create(ctx, area.ID, nil, "Butter", "250 g", "")
	require.NoError(t, err)

	results, err := items.Search(ctx, "milk")
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestItemStoreSearch_CaseInsensitive(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Fridge")
	require.NoError(t, err)
	_, err = items.Create(ctx, area.ID, nil, "Orange Juice", "1 liter", "")
	require.NoError(t, err)

	results, err := items.Search(ctx, "ORANGE")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Orange Juice", results[0].Name)
}

func TestItemStoreSearch_NoMatch(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Fridge")
	require.NoError(t, err)
	_, err = items.Create(ctx, area.ID, nil, "Cheese", "1 block", "")
	require.NoError(t, err)

	results, err := items.Search(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, results)
}

// Regression test for kitchinv-foy: search must not return items whose area
// has been deleted.
func TestItemStoreSearch_DeletedArea(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "ToDelete")
	require.NoError(t, err)
	_, err = items.Create(ctx, area.ID, nil, "Milk", "1 liter", "")
	require.NoError(t, err)

	// Delete the area — items should cascade-delete.
	require.NoError(t, areas.Delete(ctx, area.ID))

	results, err := items.Search(ctx, "Milk")
	require.NoError(t, err)
	assert.Empty(t, results, "search must not return items from deleted areas")
}

func TestItemStoreUpdate(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Fridge")
	require.NoError(t, err)

	item, err := items.Create(ctx, area.ID, nil, "Milk", "1 liter", "opened")
	require.NoError(t, err)

	err = items.Update(ctx, item.ID, "Whole Milk", "2 liters", "fresh")
	require.NoError(t, err)

	updated, err := items.GetByID(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, "Whole Milk", updated.Name)
	assert.Equal(t, "2 liters", updated.Quantity)
	assert.Equal(t, "fresh", updated.Notes)
}

func TestItemStoreUpdate_NotFound(t *testing.T) {
	d := openTestDB(t)
	items := NewItemStore(d)
	ctx := context.Background()

	err := items.Update(ctx, 99999, "Name", "1", "")
	assert.Error(t, err)
}

func TestItemStoreDelete(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Fridge")
	require.NoError(t, err)

	item, err := items.Create(ctx, area.ID, nil, "Milk", "1 liter", "")
	require.NoError(t, err)

	err = items.Delete(ctx, item.ID)
	require.NoError(t, err)

	deleted, err := items.GetByID(ctx, item.ID)
	require.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestItemStoreDelete_NotFound(t *testing.T) {
	d := openTestDB(t)
	items := NewItemStore(d)
	ctx := context.Background()

	err := items.Delete(ctx, 99999)
	assert.Error(t, err)
}

func TestItemStoreDeleteByAreaID(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Freezer")
	require.NoError(t, err)

	_, err = items.Create(ctx, area.ID, nil, "Ice cream", "1 tub", "")
	require.NoError(t, err)
	_, err = items.Create(ctx, area.ID, nil, "Frozen peas", "500 g", "")
	require.NoError(t, err)

	err = items.DeleteByAreaID(ctx, area.ID)
	require.NoError(t, err)

	list, err := items.ListByAreaID(ctx, area.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}
