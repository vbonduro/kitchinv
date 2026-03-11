package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestItemEditStoreCreate(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	edits := NewItemEditStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Fridge")
	require.NoError(t, err)
	item, err := items.Create(ctx, area.ID, nil, "Milk", "1 liter", "opened", "user")
	require.NoError(t, err)

	edit, err := edits.Create(ctx, item.ID, "name", "Milk", "Whole Milk")
	require.NoError(t, err)
	assert.NotZero(t, edit.ID)
	assert.Equal(t, item.ID, edit.ItemID)
	assert.Equal(t, "name", edit.Field)
	assert.Equal(t, "Milk", edit.OldValue)
	assert.Equal(t, "Whole Milk", edit.NewValue)
	assert.False(t, edit.EditedAt.IsZero())
}

func TestItemEditStoreListByItemID(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	edits := NewItemEditStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Fridge")
	require.NoError(t, err)
	item, err := items.Create(ctx, area.ID, nil, "Milk", "1 liter", "", "user")
	require.NoError(t, err)

	_, err = edits.Create(ctx, item.ID, "name", "Milk", "Whole Milk")
	require.NoError(t, err)
	_, err = edits.Create(ctx, item.ID, "quantity", "1 liter", "2 liters")
	require.NoError(t, err)

	list, err := edits.ListByItemID(ctx, item.ID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestItemEditStoreListByItemID_CascadeDelete(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	items := NewItemStore(d)
	edits := NewItemEditStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Fridge")
	require.NoError(t, err)
	item, err := items.Create(ctx, area.ID, nil, "Milk", "1 liter", "", "user")
	require.NoError(t, err)

	_, err = edits.Create(ctx, item.ID, "name", "Milk", "Whole Milk")
	require.NoError(t, err)

	require.NoError(t, items.Delete(ctx, item.ID))

	list, err := edits.ListByItemID(ctx, item.ID)
	require.NoError(t, err)
	assert.Empty(t, list, "edits should be cascade-deleted with the item")
}
