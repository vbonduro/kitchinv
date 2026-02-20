package local

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalPhotoStoreSaveAndGet(t *testing.T) {
	tmpdir := t.TempDir()
	store, err := NewLocalPhotoStore(tmpdir)
	require.NoError(t, err)

	ctx := context.Background()
	imageData := []byte("fake jpeg data")

	// Save
	key, err := store.Save(ctx, "area_1", "image/jpeg", bytes.NewReader(imageData))
	require.NoError(t, err)
	assert.NotEmpty(t, key)

	// Get
	reader, mimeType, err := store.Get(ctx, key)
	require.NoError(t, err)
	defer reader.Close()

	assert.Equal(t, "image/jpeg", mimeType)

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, imageData, data)
}

func TestLocalPhotoStoreDelete(t *testing.T) {
	tmpdir := t.TempDir()
	store, err := NewLocalPhotoStore(tmpdir)
	require.NoError(t, err)

	ctx := context.Background()
	imageData := []byte("test data")

	// Save
	key, err := store.Save(ctx, "area_1", "image/jpeg", bytes.NewReader(imageData))
	require.NoError(t, err)

	// Delete
	err = store.Delete(ctx, key)
	require.NoError(t, err)

	// Verify deleted
	_, _, err = store.Get(ctx, key)
	assert.Error(t, err)
}

func TestLocalPhotoStoreNotFound(t *testing.T) {
	tmpdir := t.TempDir()
	store, err := NewLocalPhotoStore(tmpdir)
	require.NoError(t, err)

	ctx := context.Background()

	_, _, err = store.Get(ctx, "nonexistent.jpg")
	assert.Error(t, err)
}

func TestLocalPhotoStorePathTraversal(t *testing.T) {
	tmpdir := t.TempDir()
	store, err := NewLocalPhotoStore(tmpdir)
	require.NoError(t, err)

	ctx := context.Background()

	// Try to traverse directory
	_, _, err = store.Get(ctx, "../../etc/passwd")
	assert.Error(t, err)
}
