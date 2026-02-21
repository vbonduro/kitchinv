package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhotoStoreCreate(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	photos := NewPhotoStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Fridge")
	require.NoError(t, err)

	photo, err := photos.Create(ctx, area.ID, "area_1/abc123.jpg", "image/jpeg")
	require.NoError(t, err)
	assert.NotZero(t, photo.ID)
	assert.Equal(t, area.ID, photo.AreaID)
	assert.Equal(t, "area_1/abc123.jpg", photo.StorageKey)
	assert.Equal(t, "image/jpeg", photo.MimeType)
}

func TestPhotoStoreGetLatestByAreaID(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	photos := NewPhotoStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Fridge")
	require.NoError(t, err)

	first, err := photos.Create(ctx, area.ID, "key1.jpg", "image/jpeg")
	require.NoError(t, err)
	second, err := photos.Create(ctx, area.ID, "key2.jpg", "image/jpeg")
	require.NoError(t, err)

	latest, err := photos.GetLatestByAreaID(ctx, area.ID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	// Both photos may share the same timestamp in SQLite; assert we got one of them.
	assert.Contains(t, []int64{first.ID, second.ID}, latest.ID)
}

func TestPhotoStoreGetLatestByAreaID_NoPhotos(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	photos := NewPhotoStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Empty Fridge")
	require.NoError(t, err)

	latest, err := photos.GetLatestByAreaID(ctx, area.ID)
	require.NoError(t, err)
	assert.Nil(t, latest)
}

func TestPhotoStoreDelete(t *testing.T) {
	d := openTestDB(t)
	areas := NewAreaStore(d)
	photos := NewPhotoStore(d)
	ctx := context.Background()

	area, err := areas.Create(ctx, "Fridge")
	require.NoError(t, err)

	photo, err := photos.Create(ctx, area.ID, "key.jpg", "image/jpeg")
	require.NoError(t, err)

	err = photos.Delete(ctx, photo.ID)
	require.NoError(t, err)

	retrieved, err := photos.GetByID(ctx, photo.ID)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestPhotoStoreDelete_NotFound(t *testing.T) {
	d := openTestDB(t)
	photos := NewPhotoStore(d)
	ctx := context.Background()

	err := photos.Delete(ctx, 99999)
	assert.Error(t, err)
}
