package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbonduro/kitchinv/internal/domain"
	"github.com/vbonduro/kitchinv/internal/service"
	"github.com/vbonduro/kitchinv/internal/web/templates"
	"log/slog"
)

// fakeDBService is a minimal kitchenService stub for the api/db tests.
type fakeDBService struct {
	summaries    []*service.AreaSummary
	summariesErr error
}

func (f *fakeDBService) ListAreasWithItems(_ context.Context) ([]*service.AreaSummary, error) {
	return f.summaries, f.summariesErr
}

// Remaining kitchenService stubs.
func (f *fakeDBService) ListAreas(_ context.Context) ([]*domain.Area, error)       { return nil, nil }
func (f *fakeDBService) CreateArea(_ context.Context, _ string) (*domain.Area, error) {
	return nil, nil
}
func (f *fakeDBService) GetArea(_ context.Context, _ int64) (*domain.Area, error) { return nil, nil }
func (f *fakeDBService) GetAreaWithItems(_ context.Context, _ int64) (*domain.Area, []*domain.Item, *domain.Photo, error) {
	return nil, nil, nil, nil
}
func (f *fakeDBService) UpdateArea(_ context.Context, _ int64, _ string) (*domain.Area, error) {
	return nil, nil
}
func (f *fakeDBService) DeleteArea(_ context.Context, _ int64) error  { return nil }
func (f *fakeDBService) DeletePhoto(_ context.Context, _ int64) error { return nil }
func (f *fakeDBService) UploadPhoto(_ context.Context, _ int64, _ []byte, _ string) (*domain.Photo, []*domain.Item, error) {
	return nil, nil, nil
}
func (f *fakeDBService) CreateItem(_ context.Context, _ int64, _, _ string) (*domain.Item, error) {
	return nil, nil
}
func (f *fakeDBService) UpdateItem(_ context.Context, _ int64, _, _ string) (*domain.Item, error) {
	return nil, nil
}
func (f *fakeDBService) DeleteItem(_ context.Context, _ int64) error      { return nil }
func (f *fakeDBService) ReorderAreas(_ context.Context, _ []int64) error  { return nil }
func (f *fakeDBService) SearchItems(_ context.Context, _ string) ([]*domain.Item, error) {
	return nil, nil
}
func (f *fakeDBService) ListSnapshots(_ context.Context, _ int64) ([]*domain.Snapshot, error) {
	return nil, nil
}
func (f *fakeDBService) ListOverrideRules(_ context.Context) ([]*domain.OverrideRule, error) {
	return nil, nil
}
func (f *fakeDBService) CreateOverrideRule(_ context.Context, r domain.OverrideRule) (*domain.OverrideRule, error) {
	return nil, nil
}
func (f *fakeDBService) GetOverrideRule(_ context.Context, _ int64) (*domain.OverrideRule, error) {
	return nil, nil
}
func (f *fakeDBService) UpdateOverrideRule(_ context.Context, r domain.OverrideRule) (*domain.OverrideRule, error) {
	return nil, nil
}
func (f *fakeDBService) DeleteOverrideRule(_ context.Context, _ int64) error      { return nil }
func (f *fakeDBService) ReorderOverrideRules(_ context.Context, _ []int64) error  { return nil }

func newDBAPITestServer(svc kitchenService, dbPath string) *Server {
	return NewServer(svc, templates.FS, nil, slog.Default(), dbPath)
}

// --- /api/db/hash tests ---

func TestHandleAPIDBHash_OK(t *testing.T) {
	content := []byte("fake sqlite database content")
	tmp, err := os.CreateTemp(t.TempDir(), "kitchinv*.db")
	require.NoError(t, err)
	_, err = tmp.Write(content)
	require.NoError(t, err)
	require.NoError(t, tmp.Close())

	want := sha256.Sum256(content)
	wantHex := hex.EncodeToString(want[:])

	srv := newDBAPITestServer(&fakeDBService{}, tmp.Name())
	req := httptest.NewRequest("GET", "/api/db/hash", nil)
	rec := httptest.NewRecorder()
	srv.handleAPIDBHash(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got struct {
		Hash string `json:"hash"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	assert.Equal(t, wantHex, got.Hash)
}

func TestHandleAPIDBHash_FileNotFound(t *testing.T) {
	srv := newDBAPITestServer(&fakeDBService{}, "/nonexistent/path/to.db")
	req := httptest.NewRequest("GET", "/api/db/hash", nil)
	rec := httptest.NewRecorder()
	srv.handleAPIDBHash(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- /api/db tests ---

func TestHandleAPIDB_OK(t *testing.T) {
	svc := &fakeDBService{
		summaries: []*service.AreaSummary{
			{
				Area: &domain.Area{ID: 1, Name: "Fridge"},
				Items: []*domain.Item{
					{Name: "Milk", Quantity: "2"},
					{Name: "Butter", Quantity: "1"},
				},
			},
			{
				Area:  &domain.Area{ID: 2, Name: "Pantry"},
				Items: []*domain.Item{},
			},
		},
	}
	srv := newDBAPITestServer(svc, "")
	req := httptest.NewRequest("GET", "/api/db", nil)
	rec := httptest.NewRecorder()
	srv.handleAPIDB(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got struct {
		Areas []struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			Items []struct {
				Name     string `json:"Name"`
				Quantity string `json:"Quantity"`
			} `json:"items"`
		} `json:"areas"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	require.Len(t, got.Areas, 2)
	assert.Equal(t, int64(1), got.Areas[0].ID)
	assert.Equal(t, "Fridge", got.Areas[0].Name)
	require.Len(t, got.Areas[0].Items, 2)
	assert.Equal(t, "Milk", got.Areas[0].Items[0].Name)
	assert.Equal(t, "2", got.Areas[0].Items[0].Quantity)
	assert.Equal(t, int64(2), got.Areas[1].ID)
	assert.Empty(t, got.Areas[1].Items)
}

func TestHandleAPIDB_Empty(t *testing.T) {
	svc := &fakeDBService{summaries: []*service.AreaSummary{}}
	srv := newDBAPITestServer(svc, "")
	req := httptest.NewRequest("GET", "/api/db", nil)
	rec := httptest.NewRecorder()
	srv.handleAPIDB(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var got struct {
		Areas []interface{} `json:"areas"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	assert.Empty(t, got.Areas)
}

func TestHandleAPIDB_ServiceError(t *testing.T) {
	svc := &fakeDBService{summariesErr: errors.New("db down")}
	srv := newDBAPITestServer(svc, "")
	req := httptest.NewRequest("GET", "/api/db", nil)
	rec := httptest.NewRecorder()
	srv.handleAPIDB(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
