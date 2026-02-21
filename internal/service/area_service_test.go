package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbonduro/kitchinv/internal/db"
	"github.com/vbonduro/kitchinv/internal/store"
	"github.com/vbonduro/kitchinv/internal/vision"
)

// stubVision is a minimal VisionAnalyzer for tests.
type stubVision struct {
	result *vision.AnalysisResult
	err    error
}

func (s *stubVision) Analyze(_ context.Context, _ io.Reader, _ string) (*vision.AnalysisResult, error) {
	return s.result, s.err
}

// stubPhotoStore is a minimal in-memory photostore.PhotoStore for tests.
type stubPhotoStore struct {
	saved  map[string][]byte
	saveErr error
}

func newStubPhotoStore() *stubPhotoStore {
	return &stubPhotoStore{saved: make(map[string][]byte)}
}

func (s *stubPhotoStore) Save(_ context.Context, prefix, _ string, r io.Reader) (string, error) {
	if s.saveErr != nil {
		return "", s.saveErr
	}
	data, _ := io.ReadAll(r)
	key := prefix + "/photo.jpg"
	s.saved[key] = data
	return key, nil
}

func (s *stubPhotoStore) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	data, ok := s.saved[key]
	if !ok {
		return nil, "", errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), "image/jpeg", nil
}

func (s *stubPhotoStore) Delete(_ context.Context, key string) error {
	delete(s.saved, key)
	return nil
}

func newTestService(t *testing.T) (*AreaService, func()) {
	t.Helper()
	d, err := db.OpenForTesting()
	require.NoError(t, err)

	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		&stubVision{result: &vision.AnalysisResult{}},
		newStubPhotoStore(),
	)

	return svc, func() { _ = d.Close() }
}

func TestAreaServiceCreateArea(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	area, err := svc.CreateArea(context.Background(), "Garage Fridge")
	require.NoError(t, err)
	assert.NotZero(t, area.ID)
	assert.Equal(t, "Garage Fridge", area.Name)
}

func TestAreaServiceListAreas(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	_, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)
	_, err = svc.CreateArea(ctx, "Freezer")
	require.NoError(t, err)

	areas, err := svc.ListAreas(ctx)
	require.NoError(t, err)
	assert.Len(t, areas, 2)
}

func TestAreaServiceDeleteArea(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Temp")
	require.NoError(t, err)

	err = svc.DeleteArea(ctx, area.ID)
	require.NoError(t, err)

	retrieved, err := svc.GetArea(ctx, area.ID)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestAreaServiceUploadPhoto_StoresItemsFromVision(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	defer d.Close()

	visionResult := &vision.AnalysisResult{
		Items: []vision.DetectedItem{
			{Name: "Milk", Quantity: "1 liter", Notes: "opened"},
			{Name: "Butter", Quantity: "250 g", Notes: ""},
		},
	}

	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		&stubVision{result: visionResult},
		newStubPhotoStore(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	photo, items, err := svc.UploadPhoto(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	require.NoError(t, err)
	assert.NotNil(t, photo)
	assert.Len(t, items, 2)
	assert.Equal(t, "Milk", items[0].Name) // items returned in vision order, not sorted
	assert.Equal(t, "Butter", items[1].Name)
}

func TestAreaServiceUploadPhoto_ReplacesExistingItems(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	defer d.Close()

	photoStg := newStubPhotoStore()
	areaStore := store.NewAreaStore(d)
	photoStore := store.NewPhotoStore(d)
	itemStore := store.NewItemStore(d)
	ctx := context.Background()

	firstVision := &stubVision{result: &vision.AnalysisResult{
		Items: []vision.DetectedItem{{Name: "Old Item", Quantity: "1", Notes: ""}},
	}}
	svc := NewAreaService(areaStore, photoStore, itemStore, firstVision, photoStg)

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	_, _, err = svc.UploadPhoto(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	require.NoError(t, err)

	// Upload again with different items
	svc.visionAPI = &stubVision{result: &vision.AnalysisResult{
		Items: []vision.DetectedItem{{Name: "New Item", Quantity: "2", Notes: ""}},
	}}
	_, items, err := svc.UploadPhoto(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	require.NoError(t, err)

	assert.Len(t, items, 1)
	assert.Equal(t, "New Item", items[0].Name)
}

func TestAreaServiceUploadPhoto_AreaNotFound(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	_, _, err := svc.UploadPhoto(context.Background(), 99999, []byte{0xFF, 0xD8}, "image/jpeg")
	assert.Error(t, err)
}

func TestAreaServiceUploadPhoto_VisionError(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	defer d.Close()

	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		&stubVision{err: errors.New("vision unavailable")},
		newStubPhotoStore(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	_, _, err = svc.UploadPhoto(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	assert.Error(t, err)
}

func TestAreaServiceUploadPhoto_PhotoStorageError(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	defer d.Close()

	photoStg := &stubPhotoStore{
		saved:   make(map[string][]byte),
		saveErr: errors.New("disk full"),
	}
	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		&stubVision{result: &vision.AnalysisResult{}},
		photoStg,
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	_, _, err = svc.UploadPhoto(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	assert.Error(t, err)
}

func TestAreaServiceSearchItems(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	defer d.Close()

	visionResult := &vision.AnalysisResult{
		Items: []vision.DetectedItem{
			{Name: "Whole Milk", Quantity: "1 liter", Notes: ""},
			{Name: "Oat Milk", Quantity: "500 ml", Notes: ""},
			{Name: "Butter", Quantity: "250 g", Notes: ""},
		},
	}

	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		&stubVision{result: visionResult},
		newStubPhotoStore(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)
	_, _, err = svc.UploadPhoto(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	require.NoError(t, err)

	results, err := svc.SearchItems(ctx, "milk")
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestAreaServiceListAreasWithItems(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	defer d.Close()

	visionResult := &vision.AnalysisResult{
		Items: []vision.DetectedItem{
			{Name: "Eggs", Quantity: "12", Notes: ""},
		},
	}
	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		&stubVision{result: visionResult},
		newStubPhotoStore(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)
	_, _, err = svc.UploadPhoto(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	require.NoError(t, err)

	summaries, err := svc.ListAreasWithItems(ctx)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, "Fridge", summaries[0].Name)
	assert.Len(t, summaries[0].Items, 1)
	assert.NotNil(t, summaries[0].Photo)
}
