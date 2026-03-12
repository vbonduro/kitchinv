package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbonduro/kitchinv/internal/db"
	"github.com/vbonduro/kitchinv/internal/domain"
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

// chanVision is a VisionAnalyzer that reads results from a channel,
// allowing tests to control what each concurrent call returns.
type chanVision struct {
	ch chan *vision.AnalysisResult
}

func (c *chanVision) Analyze(_ context.Context, _ io.Reader, _ string) (*vision.AnalysisResult, error) {
	return <-c.ch, nil
}

// stubPhotoStore is a minimal in-memory photostore.PhotoStore for tests.
type stubPhotoStore struct {
	mu      sync.Mutex
	saved   map[string][]byte
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
	s.mu.Lock()
	s.saved[key] = data
	s.mu.Unlock()
	return key, nil
}

func (s *stubPhotoStore) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	s.mu.Lock()
	data, ok := s.saved[key]
	s.mu.Unlock()
	if !ok {
		return nil, "", errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), "image/jpeg", nil
}

func (s *stubPhotoStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	delete(s.saved, key)
	s.mu.Unlock()
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
		store.NewItemEditStore(d),
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
		store.NewItemEditStore(d),
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
	svc := NewAreaService(areaStore, photoStore, itemStore, store.NewItemEditStore(d), firstVision, photoStg, slog.Default())

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
		store.NewItemEditStore(d),
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

func TestAreaServiceUploadPhoto_VisionError_RollsBackPhoto(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	photoStg := newStubPhotoStore()
	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		store.NewItemEditStore(d),
		&stubVision{err: errors.New("vision unavailable")},
		photoStg,
		slog.Default(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	_, _, err = svc.UploadPhoto(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	require.Error(t, err)

	// Area should have no photo — the photo record and storage file must be
	// rolled back so the area reverts to the upload zone, not stuck analysing.
	_, _, photo, err := svc.GetAreaWithItems(ctx, area.ID)
	require.NoError(t, err)
	assert.Nil(t, photo, "photo record should be rolled back after vision failure")

	// Storage file should also be gone.
	assert.Empty(t, photoStg.saved, "photo storage file should be deleted after vision failure")
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
		store.NewItemEditStore(d),
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
		store.NewItemEditStore(d),
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
		store.NewItemEditStore(d),
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

	item, err := svc.CreateItem(ctx, area.ID, "Milk", "1 liter")
	require.NoError(t, err)
	assert.Equal(t, "Milk", item.Name)
	assert.Equal(t, "1 liter", item.Quantity)
	assert.Equal(t, domain.ItemSourceUser, item.Source)
}

func TestAreaServiceUploadPhoto_ItemsHaveAISource(t *testing.T) {
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
		store.NewItemEditStore(d),
		&stubVision{result: visionResult},
		newStubPhotoStore(),
		slog.Default(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	_, items, err := svc.UploadPhoto(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, domain.ItemSourceAI, items[0].Source)
}

func TestAreaServiceUpdateItem_RecordsEdits(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	editStore := store.NewItemEditStore(d)
	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		editStore,
		&stubVision{result: &vision.AnalysisResult{}},
		newStubPhotoStore(),
		slog.Default(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)
	item, err := svc.CreateItem(ctx, area.ID, "Milk", "1 liter")
	require.NoError(t, err)

	_, err = svc.UpdateItem(ctx, item.ID, "Whole Milk", "2 liters")
	require.NoError(t, err)

	edits, err := editStore.ListByItemID(ctx, item.ID)
	require.NoError(t, err)
	require.Len(t, edits, 2) // name and quantity changed

	assert.Equal(t, "name", edits[0].Field)
	assert.Equal(t, "Milk", edits[0].OldValue)
	assert.Equal(t, "Whole Milk", edits[0].NewValue)

	assert.Equal(t, "quantity", edits[1].Field)
	assert.Equal(t, "1 liter", edits[1].OldValue)
	assert.Equal(t, "2 liters", edits[1].NewValue)
}

func TestAreaServiceUpdateItem_NoEditsWhenUnchanged(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	editStore := store.NewItemEditStore(d)
	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		editStore,
		&stubVision{result: &vision.AnalysisResult{}},
		newStubPhotoStore(),
		slog.Default(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)
	item, err := svc.CreateItem(ctx, area.ID, "Milk", "1 liter")
	require.NoError(t, err)

	_, err = svc.UpdateItem(ctx, item.ID, "Milk", "1 liter")
	require.NoError(t, err)

	edits, err := editStore.ListByItemID(ctx, item.ID)
	require.NoError(t, err)
	assert.Empty(t, edits, "no edits should be recorded when nothing changed")
}

func TestAreaServiceUpdateItem(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	item, err := svc.CreateItem(ctx, area.ID, "Milk", "1 liter")
	require.NoError(t, err)

	updated, err := svc.UpdateItem(ctx, item.ID, "Whole Milk", "2 liters")
	require.NoError(t, err)
	assert.Equal(t, "Whole Milk", updated.Name)
	assert.Equal(t, "2 liters", updated.Quantity)
}

func TestAreaServiceDeleteItem(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	item, err := svc.CreateItem(ctx, area.ID, "Milk", "1 liter")
	require.NoError(t, err)

	err = svc.DeleteItem(ctx, item.ID)
	require.NoError(t, err)
}

func TestAreaServiceUploadPhoto_ConcurrentSameArea_DoesNotCorruptItems(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	const numUploads = 5
	cv := &chanVision{ch: make(chan *vision.AnalysisResult, numUploads)}

	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		store.NewItemEditStore(d),
		cv,
		newStubPhotoStore(),
		slog.Default(),
	)
	ctx := context.Background()

	area, err := svc.CreateArea(ctx, "Fridge")
	require.NoError(t, err)

	// Each upload will return a distinct single-item result.
	sets := make([][]vision.DetectedItem, numUploads)
	for i := range sets {
		sets[i] = []vision.DetectedItem{{Name: "Item from upload", Quantity: string(rune('A' + i))}}
	}

	var wg sync.WaitGroup
	for i := range sets {
		wg.Add(1)
		cv.ch <- &vision.AnalysisResult{Items: sets[i]}
		go func() {
			defer wg.Done()
			_, _, err := svc.UploadPhoto(ctx, area.ID, []byte{0xFF, 0xD8}, "image/jpeg")
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	// After all uploads, items must belong to exactly one upload's result set —
	// no mixing of items from different uploads.
	items, err := svc.SearchItems(ctx, "Item from upload")
	require.NoError(t, err)
	assert.Len(t, items, 1, "expected items from exactly one upload, got mixed results")
}

func TestAreaServiceUploadPhoto_ConcurrentDifferentAreas_DoNotInterfere(t *testing.T) {
	d, err := db.OpenForTesting()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	const numAreas = 5
	cv := &chanVision{ch: make(chan *vision.AnalysisResult, numAreas)}

	svc := NewAreaService(
		store.NewAreaStore(d),
		store.NewPhotoStore(d),
		store.NewItemStore(d),
		store.NewItemEditStore(d),
		cv,
		newStubPhotoStore(),
		slog.Default(),
	)
	ctx := context.Background()

	areaIDs := make([]int64, numAreas)
	for i := range areaIDs {
		a, err := svc.CreateArea(ctx, "Area "+string(rune('A'+i)))
		require.NoError(t, err)
		areaIDs[i] = a.ID
		cv.ch <- &vision.AnalysisResult{Items: []vision.DetectedItem{
			{Name: "UniqueItem" + string(rune('A'+i)), Quantity: "1"},
		}}
	}

	var wg sync.WaitGroup
	for _, id := range areaIDs {
		wg.Add(1)
		go func(areaID int64) {
			defer wg.Done()
			_, _, err := svc.UploadPhoto(ctx, areaID, []byte{0xFF, 0xD8}, "image/jpeg")
			assert.NoError(t, err)
		}(id)
	}
	wg.Wait()

	// Each area must have exactly its own item.
	for i, id := range areaIDs {
		items, err := svc.SearchItems(ctx, "UniqueItem"+string(rune('A'+i)))
		require.NoError(t, err)
		assert.Len(t, items, 1, "area %d should have exactly 1 item", id)
	}
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
		store.NewItemEditStore(d),
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
