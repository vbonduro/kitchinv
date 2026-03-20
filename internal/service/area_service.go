package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/vbonduro/kitchinv/internal/domain"
	"github.com/vbonduro/kitchinv/internal/photostore"
	"github.com/vbonduro/kitchinv/internal/vision"
)

// encodeBBoxesJSON encodes a slice of bboxes to a JSON string for DB insertion.
// Returns nil if bboxes is empty (NULL in DB).
func encodeBBoxesJSON(bboxes [][]float64) interface{} {
	if len(bboxes) == 0 {
		return nil
	}
	b, err := json.Marshal(bboxes)
	if err != nil {
		return nil
	}
	return string(b)
}

// ErrNameTaken is returned by UpdateArea when the requested name is already
// used by another area.
var ErrNameTaken = errors.New("an area with this name already exists")

// areaRepository is the subset of store.AreaStore that AreaService requires.
type areaRepository interface {
	Create(ctx context.Context, name string) (*domain.Area, error)
	GetByID(ctx context.Context, id int64) (*domain.Area, error)
	List(ctx context.Context) ([]*domain.Area, error)
	Update(ctx context.Context, id int64, name string) error
	Delete(ctx context.Context, id int64) error
	UpdateSortOrder(ctx context.Context, ids []int64) error
}

// photoRepository is the subset of store.PhotoStore that AreaService requires.
type photoRepository interface {
	Create(ctx context.Context, areaID int64, storageKey, mimeType string) (*domain.Photo, error)
	GetLatestByAreaID(ctx context.Context, areaID int64) (*domain.Photo, error)
	Delete(ctx context.Context, id int64) error
	DeleteByArea(ctx context.Context, areaID int64) (*domain.Photo, error)
}

// itemRepository is the subset of store.ItemStore that AreaService requires.
type itemRepository interface {
	Create(ctx context.Context, areaID int64, photoID *int64, name, quantity, source string, bboxes [][]float64) (*domain.Item, error)
	GetByID(ctx context.Context, id int64) (*domain.Item, error)
	ListByAreaID(ctx context.Context, areaID int64) ([]*domain.Item, error)
	Update(ctx context.Context, id int64, name, quantity string) error
	Delete(ctx context.Context, id int64) error
	DeleteByAreaID(ctx context.Context, areaID int64) error
	Search(ctx context.Context, query string) ([]*domain.Item, error)
}

// itemEditRepository is the subset of store.ItemEditStore that AreaService requires.
type itemEditRepository interface {
	Create(ctx context.Context, itemID int64, field, oldValue, newValue string) (*domain.ItemEdit, error)
	ListByItemID(ctx context.Context, itemID int64) ([]*domain.ItemEdit, error)
}

// snapshotRepository persists area inventory snapshots.
type snapshotRepository interface {
	Create(ctx context.Context, areaID int64, items []domain.SnapshotItem) (*domain.Snapshot, error)
	ListByAreaID(ctx context.Context, areaID int64) ([]*domain.Snapshot, error)
}

// overrideRepository manages item name override rules.
type overrideRepository interface {
	ListForArea(ctx context.Context, areaID int64) ([]*domain.OverrideRule, error)
	List(ctx context.Context) ([]*domain.OverrideRule, error)
	Create(ctx context.Context, r domain.OverrideRule) (*domain.OverrideRule, error)
	GetByID(ctx context.Context, id int64) (*domain.OverrideRule, error)
	Update(ctx context.Context, r domain.OverrideRule) (*domain.OverrideRule, error)
	Delete(ctx context.Context, id int64) error
	ReorderSortOrder(ctx context.Context, ids []int64) error
	ListEditSuggestions(ctx context.Context) ([]*domain.EditSuggestion, error)
}

type AreaService struct {
	areaStore      areaRepository
	photoStore     photoRepository
	itemStore      itemRepository
	itemEditStore  itemEditRepository
	snapshotStore  snapshotRepository
	overrideStore  overrideRepository
	visionAPI      vision.VisionAnalyzer
	photoStg       photostore.PhotoStore
	logger         *slog.Logger
	db             *sql.DB
	uploadLocks    sync.Map // key: int64 areaID → *sync.Mutex
}

func NewAreaService(
	areaStore areaRepository,
	photoStore photoRepository,
	itemStore itemRepository,
	itemEditStore itemEditRepository,
	snapshotStore snapshotRepository,
	overrideStore overrideRepository,
	visionAPI vision.VisionAnalyzer,
	photoStg photostore.PhotoStore,
	logger *slog.Logger,
) *AreaService {
	return &AreaService{
		areaStore:     areaStore,
		photoStore:    photoStore,
		itemStore:     itemStore,
		itemEditStore: itemEditStore,
		snapshotStore: snapshotStore,
		overrideStore: overrideStore,
		visionAPI:     visionAPI,
		photoStg:      photoStg,
		logger:        logger,
	}
}

// WithDB sets the *sql.DB used for transactional item replacement.
// Must be called before UploadPhoto is used.
func (s *AreaService) WithDB(db *sql.DB) *AreaService {
	s.db = db
	return s
}

func (s *AreaService) lockForArea(areaID int64) func() {
	v, _ := s.uploadLocks.LoadOrStore(areaID, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (s *AreaService) CreateArea(ctx context.Context, name string) (*domain.Area, error) {
	area, err := s.areaStore.Create(ctx, name)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrNameTaken
		}
		return nil, err
	}
	return area, nil
}

func (s *AreaService) ListAreas(ctx context.Context) ([]*domain.Area, error) {
	return s.areaStore.List(ctx)
}

// AreaSummary bundles an area with its latest photo and items for list rendering.
type AreaSummary struct {
	*domain.Area
	Photo *domain.Photo
	Items []*domain.Item
}

func (s *AreaService) ListAreasWithItems(ctx context.Context) ([]*AreaSummary, error) {
	areas, err := s.areaStore.List(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]*AreaSummary, 0, len(areas))
	for _, area := range areas {
		items, err := s.itemStore.ListByAreaID(ctx, area.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list items for area %d: %w", area.ID, err)
		}
		photo, err := s.photoStore.GetLatestByAreaID(ctx, area.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get photo for area %d: %w", area.ID, err)
		}
		summaries = append(summaries, &AreaSummary{Area: area, Photo: photo, Items: items})
	}
	return summaries, nil
}

func (s *AreaService) GetArea(ctx context.Context, areaID int64) (*domain.Area, error) {
	return s.areaStore.GetByID(ctx, areaID)
}

func (s *AreaService) DeleteArea(ctx context.Context, areaID int64) error {
	return s.areaStore.Delete(ctx, areaID)
}

// UploadPhoto saves the photo to storage and commits the DB record before running
// vision analysis, so a page refresh during analysis sees Photo&&!Items (analysing
// state) and resumes polling. Concurrent calls for the same areaID are serialised;
// concurrent calls for different areas run in parallel.
func (s *AreaService) UploadPhoto(ctx context.Context, areaID int64, imageData []byte, mimeType string) (*domain.Photo, []*domain.Item, error) {
	s.logger.Info("upload photo started", "area_id", areaID, "mime_type", mimeType, "bytes", len(imageData))

	area, err := s.areaStore.GetByID(ctx, areaID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get area: %w", err)
	}
	if area == nil {
		return nil, nil, fmt.Errorf("area not found")
	}

	// Save photo and commit the DB record before calling the vision API so
	// that a client disconnect/refresh sees Photo&&!Items and polls for results.
	storageKey, err := s.photoStg.Save(ctx, fmt.Sprintf("area_%d", areaID), mimeType, bytes.NewReader(imageData))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save photo: %w", err)
	}
	s.logger.Debug("photo saved", "area_id", areaID, "storage_key", storageKey)

	// Serialise the write sequence per area: create photo record, delete old
	// items, insert new items. This prevents concurrent uploads to the same
	// area from interleaving their delete+insert sequences and corrupting data.
	unlock := s.lockForArea(areaID)
	defer unlock()

	photo, err := s.photoStore.Create(ctx, areaID, storageKey, mimeType)
	if err != nil {
		_ = s.photoStg.Delete(ctx, storageKey)
		return nil, nil, fmt.Errorf("failed to create photo record: %w", err)
	}

	s.logger.Info("vision analysis started", "area_id", areaID)
	result, err := s.visionAPI.Analyze(ctx, bytes.NewReader(imageData), mimeType)
	if err != nil {
		// Roll back the photo record and storage file so the area reverts to
		// the upload zone rather than being stuck in the analysing state.
		if delErr := s.photoStore.Delete(ctx, photo.ID); delErr != nil {
			s.logger.Error("failed to delete photo record after analysis failure", "area_id", areaID, "error", delErr)
		}
		_ = s.photoStg.Delete(ctx, storageKey)
		return nil, nil, fmt.Errorf("failed to analyze image: %w", err)
	}
	s.logger.Info("vision analysis complete", "area_id", areaID, "status", result.Status, "items_detected", len(result.Items))
	if result.Status != vision.StatusOK && result.Status != "" {
		s.logger.Info("vision analysis non-ok result", "area_id", areaID, "status", result.Status)
	}

	items, err := s.replaceItems(ctx, areaID, photo.ID, result.Items)
	if err != nil {
		return photo, nil, err
	}

	s.logger.Info("upload photo complete", "area_id", areaID, "items_stored", len(items))
	return photo, items, nil
}

// replaceItems atomically deletes all existing items for an area and inserts the
// newly detected ones. If a *sql.DB is available it uses a transaction; otherwise
// it falls back to non-transactional execution (test environments without WithDB).
func (s *AreaService) replaceItems(ctx context.Context, areaID, photoID int64, detected []vision.DetectedItem) ([]*domain.Item, error) {
	if s.db != nil {
		return s.replaceItemsTx(ctx, areaID, photoID, detected)
	}
	// Fallback (tests without a DB reference): non-transactional but still
	// protected by the per-area lock acquired in UploadPhoto.
	if err := s.itemStore.DeleteByAreaID(ctx, areaID); err != nil {
		return nil, fmt.Errorf("failed to delete old items: %w", err)
	}
	merged := mergeDetectedItems(detected)
	merged = s.applyOverridesToMerged(ctx, areaID, merged)
	items := make([]*domain.Item, 0, len(merged))
	for _, m := range merged {
		item, err := s.itemStore.Create(ctx, areaID, &photoID, m.name, m.quantity, string(domain.ItemSourceAI), m.bboxes)
		if err != nil {
			s.logger.Error("failed to create item", "name", m.name, "error", err)
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *AreaService) replaceItemsTx(ctx context.Context, areaID, photoID int64, detected []vision.DetectedItem) ([]*domain.Item, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Snapshot the existing inventory before replacing it.
	existing, err := s.itemStore.ListByAreaID(ctx, areaID)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing items: %w", err)
	}
	if len(existing) > 0 {
		snapItems := make([]domain.SnapshotItem, len(existing))
		for i, it := range existing {
			snapItems[i] = domain.SnapshotItem{Name: it.Name, Quantity: it.Quantity}
		}
		if _, err := s.snapshotStore.Create(ctx, areaID, snapItems); err != nil {
			s.logger.Error("failed to create inventory snapshot", "area_id", areaID, "error", err)
			// Non-fatal: continue with the replacement even if snapshotting fails.
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM items WHERE area_id = ?`, areaID); err != nil {
		return nil, fmt.Errorf("failed to delete old items: %w", err)
	}

	merged := mergeDetectedItems(detected)
	merged = s.applyOverridesToMerged(ctx, areaID, merged)
	items := make([]*domain.Item, 0, len(merged))
	for _, m := range merged {
		bboxesJSON := encodeBBoxesJSON(m.bboxes)
		result, err := tx.ExecContext(ctx,
			`INSERT INTO items (area_id, photo_id, name, quantity, source, bboxes) VALUES (?, ?, ?, ?, ?, ?)`,
			areaID, photoID, m.name, m.quantity, string(domain.ItemSourceAI), bboxesJSON)
		if err != nil {
			s.logger.Error("failed to create item", "name", m.name, "error", err)
			continue
		}
		id, err := result.LastInsertId()
		if err != nil {
			s.logger.Error("failed to get item id", "name", m.name, "error", err)
			continue
		}
		items = append(items, &domain.Item{
			ID:       id,
			AreaID:   areaID,
			PhotoID:  &photoID,
			Name:     m.name,
			Quantity: m.quantity,
			Source:   domain.ItemSourceAI,
			BBoxes:   m.bboxes,
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return items, nil
}


func (s *AreaService) GetAreaWithItems(ctx context.Context, areaID int64) (*domain.Area, []*domain.Item, *domain.Photo, error) {
	area, err := s.areaStore.GetByID(ctx, areaID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get area: %w", err)
	}
	if area == nil {
		return nil, nil, nil, fmt.Errorf("area not found")
	}

	items, err := s.itemStore.ListByAreaID(ctx, areaID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list items: %w", err)
	}

	photo, err := s.photoStore.GetLatestByAreaID(ctx, areaID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get photo: %w", err)
	}

	return area, items, photo, nil
}

func (s *AreaService) UpdateArea(ctx context.Context, areaID int64, name string) (*domain.Area, error) {
	if err := s.areaStore.Update(ctx, areaID, name); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrNameTaken
		}
		return nil, fmt.Errorf("failed to update area: %w", err)
	}
	return s.areaStore.GetByID(ctx, areaID)
}

func (s *AreaService) DeletePhoto(ctx context.Context, areaID int64) error {
	photo, err := s.photoStore.DeleteByArea(ctx, areaID)
	if err != nil {
		return fmt.Errorf("failed to delete photo record: %w", err)
	}
	if photo == nil {
		return nil
	}

	if err := s.itemStore.DeleteByAreaID(ctx, areaID); err != nil {
		return fmt.Errorf("failed to delete items: %w", err)
	}

	if err := s.photoStg.Delete(ctx, photo.StorageKey); err != nil {
		s.logger.Error("failed to delete photo file", "storage_key", photo.StorageKey, "error", err)
	}

	return nil
}

func (s *AreaService) CreateItem(ctx context.Context, areaID int64, name, quantity string) (*domain.Item, error) {
	return s.itemStore.Create(ctx, areaID, nil, name, quantity, string(domain.ItemSourceUser), nil)
}


func (s *AreaService) UpdateItem(ctx context.Context, itemID int64, name, quantity string) (*domain.Item, error) {
	old, err := s.itemStore.GetByID(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	if old == nil {
		return nil, fmt.Errorf("item not found")
	}

	if err := s.itemStore.Update(ctx, itemID, name, quantity); err != nil {
		return nil, fmt.Errorf("failed to update item: %w", err)
	}

	// Record a diff entry for each changed field.
	for _, change := range []struct{ field, oldVal, newVal string }{
		{"name", old.Name, name},
		{"quantity", old.Quantity, quantity},
	} {
		if change.oldVal != change.newVal {
			if _, err := s.itemEditStore.Create(ctx, itemID, change.field, change.oldVal, change.newVal); err != nil {
				s.logger.Error("failed to record item edit", "item_id", itemID, "field", change.field, "error", err)
			}
		}
	}

	return s.itemStore.GetByID(ctx, itemID)
}

func (s *AreaService) DeleteItem(ctx context.Context, itemID int64) error {
	return s.itemStore.Delete(ctx, itemID)
}

func (s *AreaService) ReorderAreas(ctx context.Context, ids []int64) error {
	return s.areaStore.UpdateSortOrder(ctx, ids)
}

func (s *AreaService) SearchItems(ctx context.Context, query string) ([]*domain.Item, error) {
	return s.itemStore.Search(ctx, query)
}

func (s *AreaService) ListSnapshots(ctx context.Context, areaID int64) ([]*domain.Snapshot, error) {
	return s.snapshotStore.ListByAreaID(ctx, areaID)
}

// applyOverridesToMerged loads override rules for an area and applies them,
// dropping items whose name becomes empty after substitution.
func (s *AreaService) applyOverridesToMerged(ctx context.Context, areaID int64, merged []mergedItem) []mergedItem {
	if s.overrideStore == nil {
		return merged
	}
	rules, err := s.overrideStore.ListForArea(ctx, areaID)
	if err != nil {
		s.logger.Error("failed to load override rules", "area_id", areaID, "error", err)
		return merged // non-fatal: proceed with original names
	}
	filtered := merged[:0]
	for _, m := range merged {
		m.name = applyOverrides(rules, m.name)
		if strings.TrimSpace(m.name) != "" {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// ListOverrideRules returns all override rules.
func (s *AreaService) ListOverrideRules(ctx context.Context) ([]*domain.OverrideRule, error) {
	return s.overrideStore.List(ctx)
}

// CreateOverrideRule creates a new override rule.
func (s *AreaService) CreateOverrideRule(ctx context.Context, r domain.OverrideRule) (*domain.OverrideRule, error) {
	return s.overrideStore.Create(ctx, r)
}

// GetOverrideRule fetches an override rule by ID.
func (s *AreaService) GetOverrideRule(ctx context.Context, id int64) (*domain.OverrideRule, error) {
	return s.overrideStore.GetByID(ctx, id)
}

// UpdateOverrideRule updates an existing override rule.
func (s *AreaService) UpdateOverrideRule(ctx context.Context, r domain.OverrideRule) (*domain.OverrideRule, error) {
	return s.overrideStore.Update(ctx, r)
}

// DeleteOverrideRule deletes an override rule.
func (s *AreaService) DeleteOverrideRule(ctx context.Context, id int64) error {
	return s.overrideStore.Delete(ctx, id)
}

// ReorderOverrideRules sets sort_order based on the provided ID order.
func (s *AreaService) ReorderOverrideRules(ctx context.Context, ids []int64) error {
	return s.overrideStore.ReorderSortOrder(ctx, ids)
}

// ListEditSuggestions returns recent name renames as override rule suggestions.
func (s *AreaService) ListEditSuggestions(ctx context.Context) ([]*domain.EditSuggestion, error) {
	return s.overrideStore.ListEditSuggestions(ctx)
}
