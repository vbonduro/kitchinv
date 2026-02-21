package service

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/vbonduro/kitchinv/internal/domain"
	"github.com/vbonduro/kitchinv/internal/photostore"
	"github.com/vbonduro/kitchinv/internal/store"
	"github.com/vbonduro/kitchinv/internal/vision"
)

type AreaService struct {
	areaStore  *store.AreaStore
	photoStore *store.PhotoStore
	itemStore  *store.ItemStore
	visionAPI  vision.VisionAnalyzer
	photoStg   photostore.PhotoStore
}

func NewAreaService(
	areaStore *store.AreaStore,
	photoStore *store.PhotoStore,
	itemStore *store.ItemStore,
	visionAPI vision.VisionAnalyzer,
	photoStg photostore.PhotoStore,
) *AreaService {
	return &AreaService{
		areaStore:  areaStore,
		photoStore: photoStore,
		itemStore:  itemStore,
		visionAPI:  visionAPI,
		photoStg:   photoStg,
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
	area, err := s.areaStore.GetByID(ctx, areaID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get area: %w", err)
	}
	if area == nil {
		return nil, nil, fmt.Errorf("area not found")
	}

	result, err := s.visionAPI.Analyze(ctx, bytes.NewReader(imageData), mimeType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to analyze image: %w", err)
	}

	storageKey, err := s.photoStg.Save(ctx, fmt.Sprintf("area_%d", areaID), mimeType, bytes.NewReader(imageData))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save photo: %w", err)
	}

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
			log.Printf("failed to create item %q: %v", detected.Name, err)
			continue
		}
		items = append(items, item)
	}

	return photo, items, nil
}

// UploadPhotoStream saves the photo, clears old items, then streams detected
// items back via the returned channel as the vision model produces them.
// The caller must drain and close the channel (it is closed by the goroutine).
func (s *AreaService) UploadPhotoStream(ctx context.Context, areaID int64, imageData []byte, mimeType string) (*domain.Photo, <-chan vision.StreamEvent, error) {
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

	photo, err := s.photoStore.Create(ctx, areaID, storageKey, mimeType)
	if err != nil {
		_ = s.photoStg.Delete(ctx, storageKey)
		return nil, nil, fmt.Errorf("failed to create photo record: %w", err)
	}

	if err := s.itemStore.DeleteByAreaID(ctx, areaID); err != nil {
		return photo, nil, fmt.Errorf("failed to delete old items: %w", err)
	}

	rawCh, err := sa.AnalyzeStream(ctx, bytes.NewReader(imageData), mimeType)
	if err != nil {
		return photo, nil, fmt.Errorf("failed to start vision stream: %w", err)
	}

	out := make(chan vision.StreamEvent, 16)
	go func() {
		defer close(out)
		for ev := range rawCh {
			if ev.Err != nil {
				out <- ev
				return
			}
			item, err := s.itemStore.Create(ctx, areaID, &photo.ID, ev.Item.Name, ev.Item.Quantity, ev.Item.Notes)
			if err != nil {
				log.Printf("failed to create item %q: %v", ev.Item.Name, err)
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

func (s *AreaService) SearchItems(ctx context.Context, query string) ([]*domain.Item, error) {
	return s.itemStore.Search(ctx, query)
}
