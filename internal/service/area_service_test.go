package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbonduro/kitchinv/internal/db"
	"github.com/vbonduro/kitchinv/internal/store"
	"github.com/vbonduro/kitchinv/internal/vision"
)

// stubVision is a minimal VisionAnalyzer for tests.
type stubVision struct {
	result    *vision.AnalysisResult
	err       error
	streamErr error // error returned from AnalyzeStream call itself
}

func (s *stubVision) Analyze(_ context.Context, _ io.Reader, _ string) (*vision.AnalysisResult, error) {
	return s.result, s.err
}

func (s *stubVision) AnalyzeStream(_ context.Context, _ io.Reader, _ string) (<-chan vision.StreamEvent, error) {
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	ch := make(chan vision.StreamEvent, len(s.result.Items)+1)
	for i := range s.result.Items {
		ch <- vision.StreamEvent{Item: &s.result.Items[i]}
	}
	close(ch)
	return ch, s.err
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
		slog.Default(),
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
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

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
		slog.Default(),
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
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	photoStg := newStubPhotoStore()
	areaStore := store.NewAreaStore(d)
	photoStore := store.NewPhotoStore(d)
	itemStore := store.NewItemStore(d)
	ctx := context.Background()

	firstVision := &stubVision{result: &vision.AnalysisResult{
		Items: []vision.DetectedItem{{Name: "Old Item", Quantity: "1", Notes: ""}},
	}}
	svc := NewAreaService(areaStore, photoStore, itemStore, firstVision, photoStg, slog.Default())

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
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		&stubVision{err: errors.New("vision unavailable")},
		newStubPhotoStore(),
		slog.Default(),
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
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

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
		slog.Default(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	_, _, err = svc.UploadPhoto(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	assert.Error(t, err)
}

func TestAreaServiceUploadPhotoStream_StreamsItems(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

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
		slog.Default(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	photo, ch, err := svc.UploadPhotoStream(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	require.NoError(t, err)
	assert.NotNil(t, photo)

	var names []string
	for ev := range ch {
		require.NoError(t, ev.Err)
		names = append(names, ev.Item.Name)
	}
	assert.Equal(t, []string{"Milk", "Butter"}, names)
}

func TestAreaServiceUploadPhotoStream_AreaNotFound(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	_, _, err := svc.UploadPhotoStream(context.Background(), 99999, []byte{0xFF, 0xD8}, "image/jpeg")
	assert.Error(t, err)
}

func TestAreaServiceUploadPhotoStream_VisionStreamError(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		&stubVision{result: &vision.AnalysisResult{}, streamErr: errors.New("stream unavailable")},
		newStubPhotoStore(),
		slog.Default(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	_, _, err = svc.UploadPhotoStream(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	assert.Error(t, err)
}

// TestAreaServiceUploadPhotoStream_AnalysisFailure_PreservesExistingState is a
// regression test for kitchinv-uh7. When AnalyzeStream fails on an area that
// already has a photo and items, the previous photo and items must be fully
// preserved — nothing should be deleted.
func TestAreaServiceUploadPhotoStream_AnalysisFailure_PreservesExistingState(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	photoStg := newStubPhotoStore()
	areaStore := store.NewAreaStore(d)
	photoStore := store.NewPhotoStore(d)
	itemStore := store.NewItemStore(d)
	ctx := context.Background()

	// First upload succeeds.
	svc := NewAreaService(areaStore, photoStore, itemStore,
		&stubVision{result: &vision.AnalysisResult{
			Items: []vision.DetectedItem{{Name: "Milk", Quantity: "1 litre", Notes: ""}},
		}},
		photoStg, slog.Default(),
	)
	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	photo, ch, err := svc.UploadPhotoStream(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	require.NoError(t, err)
	for range ch {
	} // drain
	firstPhotoID := photo.ID

	// Verify item exists after first upload.
	items, err := itemStore.ListByAreaID(ctx, area.ID)
	require.NoError(t, err)
	require.Len(t, items, 1, "expected 1 item after first upload")
	assert.Equal(t, "Milk", items[0].Name)

	// Second upload fails at AnalyzeStream.
	svc.visionAPI = &stubVision{
		result:    &vision.AnalysisResult{},
		streamErr: errors.New("image exceeds 5 MB maximum"),
	}
	_, _, err = svc.UploadPhotoStream(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	assert.Error(t, err)

	// Previous photo must still exist — the area must not be stuck in
	// "analysing" state with no photo and no way to re-upload.
	prevPhoto, err := photoStore.GetLatestByAreaID(ctx, area.ID)
	require.NoError(t, err)
	require.NotNil(t, prevPhoto, "regression(kitchinv-uh7): previous photo was deleted after failed upload")
	assert.Equal(t, firstPhotoID, prevPhoto.ID, "regression(kitchinv-uh7): photo was replaced after failed upload")

	// Previous items must be restored so the area does not show the analysing
	// overlay (photo+no items) after a page refresh.
	itemsAfter, err := itemStore.ListByAreaID(ctx, area.ID)
	require.NoError(t, err)
	require.Len(t, itemsAfter, 1, "regression(kitchinv-uh7): items not restored after failed upload")
	assert.Equal(t, "Milk", itemsAfter[0].Name)
}

func TestAreaServiceSearchItems(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

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
		slog.Default(),
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

func TestAreaServiceUpdateArea(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Old Name")
	require.NoError(t, err)

	updated, err := svc.UpdateArea(ctx, area.ID, "New Name")
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
}

func TestAreaServiceUpdateArea_DuplicateName_ReturnsErrNameTaken(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	_, err := svc.CreateArea(ctx, "Area One")
	require.NoError(t, err)

	areaTwo, err := svc.CreateArea(ctx, "Area Two")
	require.NoError(t, err)

	// Renaming Area Two to "Area One" should return ErrNameTaken.
	_, err = svc.UpdateArea(ctx, areaTwo.ID, "Area One")
	require.ErrorIs(t, err, ErrNameTaken)
}

func TestAreaServiceDeletePhoto(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	visionResult := &vision.AnalysisResult{
		Items: []vision.DetectedItem{
			{Name: "Milk", Quantity: "1 liter", Notes: ""},
		},
	}

	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		&stubVision{result: visionResult},
		newStubPhotoStore(),
		slog.Default(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	_, _, err = svc.UploadPhoto(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	require.NoError(t, err)

	err = svc.DeletePhoto(ctx, area.ID)
	require.NoError(t, err)

	// Area still exists but has no items or photo.
	_, items, photo, err := svc.GetAreaWithItems(ctx, area.ID)
	require.NoError(t, err)
	assert.Nil(t, photo)
	assert.Empty(t, items)
}

func TestAreaServiceCreateItem(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	item, err := svc.CreateItem(ctx, area.ID, "Milk", "1 liter", "opened")
	require.NoError(t, err)
	assert.Equal(t, "Milk", item.Name)
	assert.Equal(t, "1 liter", item.Quantity)
	assert.Equal(t, "opened", item.Notes)
}

func TestAreaServiceUpdateItem(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	item, err := svc.CreateItem(ctx, area.ID, "Milk", "1 liter", "opened")
	require.NoError(t, err)

	updated, err := svc.UpdateItem(ctx, item.ID, "Whole Milk", "2 liters", "fresh")
	require.NoError(t, err)
	assert.Equal(t, "Whole Milk", updated.Name)
	assert.Equal(t, "2 liters", updated.Quantity)
	assert.Equal(t, "fresh", updated.Notes)
}

func TestAreaServiceDeleteItem(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	item, err := svc.CreateItem(ctx, area.ID, "Milk", "1 liter", "")
	require.NoError(t, err)

	err = svc.DeleteItem(ctx, item.ID)
	require.NoError(t, err)
}

func TestAreaServiceListAreasWithItems(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

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
		slog.Default(),
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
