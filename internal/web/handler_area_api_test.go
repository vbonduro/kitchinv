package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbonduro/kitchinv/internal/domain"
	"github.com/vbonduro/kitchinv/internal/service"
	"github.com/vbonduro/kitchinv/internal/web/templates"
	"log/slog"
)

// fakeAreaAPIService implements kitchenService for the API area list tests.
type fakeAreaAPIService struct {
	areas    []*domain.Area
	listErr  error
}

func (f *fakeAreaAPIService) ListAreas(_ context.Context) ([]*domain.Area, error) {
	return f.areas, f.listErr
}

// Remaining kitchenService stubs.
func (f *fakeAreaAPIService) CreateArea(_ context.Context, _ string) (*domain.Area, error) {
	return nil, nil
}
func (f *fakeAreaAPIService) ListAreasWithItems(_ context.Context) ([]*service.AreaSummary, error) {
	return nil, nil
}
func (f *fakeAreaAPIService) GetArea(_ context.Context, _ int64) (*domain.Area, error) {
	return nil, nil
}
func (f *fakeAreaAPIService) GetAreaWithItems(_ context.Context, _ int64) (*domain.Area, []*domain.Item, *domain.Photo, error) {
	return nil, nil, nil, nil
}
func (f *fakeAreaAPIService) UpdateArea(_ context.Context, _ int64, _ string) (*domain.Area, error) {
	return nil, nil
}
func (f *fakeAreaAPIService) DeleteArea(_ context.Context, _ int64) error  { return nil }
func (f *fakeAreaAPIService) DeletePhoto(_ context.Context, _ int64) error { return nil }
func (f *fakeAreaAPIService) UploadPhoto(_ context.Context, _ int64, _ []byte, _ string) (*domain.Photo, []*domain.Item, error) {
	return nil, nil, nil
}
func (f *fakeAreaAPIService) CreateItem(_ context.Context, _ int64, _, _ string) (*domain.Item, error) {
	return nil, nil
}
func (f *fakeAreaAPIService) UpdateItem(_ context.Context, _ int64, _, _ string) (*domain.Item, error) {
	return nil, nil
}
func (f *fakeAreaAPIService) DeleteItem(_ context.Context, _ int64) error    { return nil }
func (f *fakeAreaAPIService) ReorderAreas(_ context.Context, _ []int64) error { return nil }
func (f *fakeAreaAPIService) SearchItems(_ context.Context, _ string) ([]*domain.Item, error) {
	return nil, nil
}
func (f *fakeAreaAPIService) ListSnapshots(_ context.Context, _ int64) ([]*domain.Snapshot, error) {
	return nil, nil
}
func (f *fakeAreaAPIService) ListOverrideRules(_ context.Context) ([]*domain.OverrideRule, error) {
	return nil, nil
}
func (f *fakeAreaAPIService) CreateOverrideRule(_ context.Context, r domain.OverrideRule) (*domain.OverrideRule, error) {
	return nil, nil
}
func (f *fakeAreaAPIService) GetOverrideRule(_ context.Context, _ int64) (*domain.OverrideRule, error) {
	return nil, nil
}
func (f *fakeAreaAPIService) UpdateOverrideRule(_ context.Context, r domain.OverrideRule) (*domain.OverrideRule, error) {
	return nil, nil
}
func (f *fakeAreaAPIService) DeleteOverrideRule(_ context.Context, _ int64) error { return nil }
func (f *fakeAreaAPIService) ReorderOverrideRules(_ context.Context, _ []int64) error {
	return nil
}

func newAreaAPITestServer(svc kitchenService) *Server {
	return NewServer(svc, templates.FS, nil, slog.Default())
}

func TestHandleAPIListAreas_OK(t *testing.T) {
	svc := &fakeAreaAPIService{
		areas: []*domain.Area{
			{ID: 1, Name: "Fridge"},
			{ID: 2, Name: "Pantry"},
		},
	}
	srv := newAreaAPITestServer(svc)

	req := httptest.NewRequest("GET", "/api/areas", nil)
	rec := httptest.NewRecorder()
	srv.handleAPIListAreas(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	require.Len(t, got, 2)
	assert.Equal(t, int64(1), got[0].ID)
	assert.Equal(t, "Fridge", got[0].Name)
	assert.Equal(t, int64(2), got[1].ID)
	assert.Equal(t, "Pantry", got[1].Name)
}

func TestHandleAPIListAreas_Empty(t *testing.T) {
	svc := &fakeAreaAPIService{areas: []*domain.Area{}}
	srv := newAreaAPITestServer(svc)

	req := httptest.NewRequest("GET", "/api/areas", nil)
	rec := httptest.NewRecorder()
	srv.handleAPIListAreas(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var got []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	assert.Empty(t, got)
}

func TestHandleAPIListAreas_ServiceError(t *testing.T) {
	svc := &fakeAreaAPIService{listErr: errors.New("db down")}
	srv := newAreaAPITestServer(svc)

	req := httptest.NewRequest("GET", "/api/areas", nil)
	rec := httptest.NewRecorder()
	srv.handleAPIListAreas(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
