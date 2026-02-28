package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"github.com/vbonduro/kitchinv/internal/domain"
	"github.com/vbonduro/kitchinv/internal/photostore"
	"github.com/vbonduro/kitchinv/internal/vision"
)

// areaRepository is the subset of store.AreaStore that AreaService requires.
type areaRepository interface {
	Create(ctx context.Context, name string) (*domain.Area, error)
	GetByID(ctx context.Context, id int64) (*domain.Area, error)
	List(ctx context.Context) ([]*domain.Area, error)
	Update(ctx context.Context, id int64, name string) error
	Delete(ctx context.Context, id int64) error
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
	Create(ctx context.Context, areaID int64, photoID *int64, name, quantity, notes string) (*domain.Item, error)
	GetByID(ctx context.Context, id int64) (*domain.Item, error)
	ListByAreaID(ctx context.Context, areaID int64) ([]*domain.Item, error)
	Update(ctx context.Context, id int64, name, quantity, notes string) error
	Delete(ctx context.Context, id int64) error
	DeleteByAreaID(ctx context.Context, areaID int64) error
	Search(ctx context.Context, query string) ([]*domain.Item, error)
}

type AreaService struct {
	areaStore  areaRepository
	photoStore photoRepository
	itemStore  itemRepository
	visionAPI  vision.VisionAnalyzer
	photoStg   photostore.PhotoStore
	logger     *slog.Logger
}

func NewAreaService(
	areaStore areaRepository,
	photoStore photoRepository,
	itemStore itemRepository,
	visionAPI vision.VisionAnalyzer,
	photoStg photostore.PhotoStore,
	logger *slog.Logger,
) *AreaService {
	return &AreaService{
		areaStore:  areaStore,
		photoStore: photoStore,
		itemStore:  itemStore,
		visionAPI:  visionAPI,
		photoStg:   photoStg,
		logger:     logger,
	}
}

func (s *AreaService) CreateArea(ctx context.Context, name string) (*domain.Area, error) {
	return s.areaStore.Create(ctx, name)
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

// UploadPhoto analyzes the image, saves it to storage, replaces the area's items, and
// returns the newly created photo record and detected items.
func (s *AreaService) UploadPhoto(ctx context.Context, areaID int64, imageData []byte, mimeType string) (*domain.Photo, []*domain.Item, error) {
	s.logger.Info("upload photo started", "area_id", areaID, "mime_type", mimeType, "bytes", len(imageData))

	area, err := s.areaStore.GetByID(ctx, areaID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get area: %w", err)
	}
	if area == nil {
		return nil, nil, fmt.Errorf("area not found")
	}

	s.logger.Info("vision analysis started", "area_id", areaID)
	result, err := s.visionAPI.Analyze(ctx, bytes.NewReader(imageData), mimeType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to analyze image: %w", err)
	}
	s.logger.Info("vision analysis complete", "area_id", areaID, "items_detected", len(result.Items))

	storageKey, err := s.photoStg.Save(ctx, fmt.Sprintf("area_%d", areaID), mimeType, bytes.NewReader(imageData))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save photo: %w", err)
	}
	s.logger.Debug("photo saved", "area_id", areaID, "storage_key", storageKey)

	photo, err := s.photoStore.Create(ctx, areaID, storageKey, mimeType)
	if err != nil {
		_ = s.photoStg.Delete(ctx, storageKey)
		return nil, nil, fmt.Errorf("failed to create photo record: %w", err)
	}

	if err := s.itemStore.DeleteByAreaID(ctx, areaID); err != nil {
		return photo, nil, fmt.Errorf("failed to delete old items: %w", err)
	}

	items := make([]*domain.Item, 0, len(result.Items))
	for _, detected := range result.Items {
		item, err := s.itemStore.Create(ctx, areaID, &photo.ID, detected.Name, detected.Quantity, detected.Notes)
		if err != nil {
			s.logger.Error("failed to create item", "name", detected.Name, "error", err)
			continue
		}
		items = append(items, item)
	}

	s.logger.Info("upload photo complete", "area_id", areaID, "items_stored", len(items))
	return photo, items, nil
}

// UploadPhotoStream saves the photo, clears old items, then streams detected
// items back via the returned channel as the vision model produces them.
// The caller must drain and close the channel (it is closed by the goroutine).
func (s *AreaService) UploadPhotoStream(ctx context.Context, areaID int64, imageData []byte, mimeType string) (*domain.Photo, <-chan vision.StreamEvent, error) {
	s.logger.Info("upload photo stream started", "area_id", areaID, "mime_type", mimeType, "bytes", len(imageData))

	area, err := s.areaStore.GetByID(ctx, areaID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get area: %w", err)
	}
	if area == nil {
		return nil, nil, fmt.Errorf("area not found")
	}

	sa, ok := s.visionAPI.(vision.StreamAnalyzer)
	if !ok {
		return nil, nil, fmt.Errorf("vision adapter does not support streaming")
	}

	storageKey, err := s.photoStg.Save(ctx, fmt.Sprintf("area_%d", areaID), mimeType, bytes.NewReader(imageData))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save photo: %w", err)
	}
	s.logger.Debug("photo saved", "area_id", areaID, "storage_key", storageKey)

	photo, err := s.photoStore.Create(ctx, areaID, storageKey, mimeType)
	if err != nil {
		_ = s.photoStg.Delete(ctx, storageKey)
		return nil, nil, fmt.Errorf("failed to create photo record: %w", err)
	}

	s.logger.Info("vision stream analysis started", "area_id", areaID)
	rawCh, err := sa.AnalyzeStream(ctx, bytes.NewReader(imageData), mimeType)
	if err != nil {
		// Roll back only the new photo record and file — do not touch the
		// previous photo or items so the area is restored to its prior state
		// (kitchinv-uh7).
		if dbErr := s.photoStore.Delete(ctx, photo.ID); dbErr != nil {
			s.logger.Error("failed to roll back photo record after stream error", "area_id", areaID, "error", dbErr)
		}
		if stgErr := s.photoStg.Delete(ctx, storageKey); stgErr != nil {
			s.logger.Error("failed to roll back photo file after stream error", "area_id", areaID, "error", stgErr)
		}
		return nil, nil, fmt.Errorf("failed to start vision stream: %w", err)
	}

	// Analysis is starting — now safe to clear old items.
	if err := s.itemStore.DeleteByAreaID(ctx, areaID); err != nil {
		return photo, nil, fmt.Errorf("failed to delete old items: %w", err)
	}

	out := make(chan vision.StreamEvent, 16)
	go func() {
		defer func() {
			s.logger.Info("vision stream analysis complete", "area_id", areaID)
			close(out)
		}()
		for ev := range rawCh {
			if ev.Err != nil {
				out <- ev
				return
			}
			s.logger.Debug("stream item detected", "area_id", areaID, "name", ev.Item.Name)
			item, err := s.itemStore.Create(ctx, areaID, &photo.ID, ev.Item.Name, ev.Item.Quantity, ev.Item.Notes)
			if err != nil {
				s.logger.Error("failed to create item", "name", ev.Item.Name, "error", err)
				continue
			}
			out <- vision.StreamEvent{Item: &vision.DetectedItem{
				Name:     item.Name,
				Quantity: item.Quantity,
				Notes:    item.Notes,
			}}
		}
	}()

	return photo, out, nil
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

func (s *AreaService) CreateItem(ctx context.Context, areaID int64, name, quantity, notes string) (*domain.Item, error) {
	return s.itemStore.Create(ctx, areaID, nil, name, quantity, notes)
}

func (s *AreaService) UpdateItem(ctx context.Context, itemID int64, name, quantity, notes string) (*domain.Item, error) {
	if err := s.itemStore.Update(ctx, itemID, name, quantity, notes); err != nil {
		return nil, fmt.Errorf("failed to update item: %w", err)
	}
	return s.itemStore.GetByID(ctx, itemID)
}

func (s *AreaService) DeleteItem(ctx context.Context, itemID int64) error {
	return s.itemStore.Delete(ctx, itemID)
}

func (s *AreaService) SearchItems(ctx context.Context, query string) ([]*domain.Item, error) {
	return s.itemStore.Search(ctx, query)
}
