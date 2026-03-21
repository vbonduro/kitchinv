package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbonduro/kitchinv/internal/domain"
	"github.com/vbonduro/kitchinv/internal/service"
	"github.com/vbonduro/kitchinv/internal/web/templates"
	"log/slog"
)

// fakeOverrideService implements kitchenService with no-ops for most methods
// and configurable override-related behaviour for testing.
type fakeOverrideService struct {
	rules     []*domain.OverrideRule
	areas     []*domain.Area
	createErr error
	updateErr error
	deleteErr error
}

func (f *fakeOverrideService) ListOverrideRules(_ context.Context) ([]*domain.OverrideRule, error) {
	return f.rules, nil
}
func (f *fakeOverrideService) CreateOverrideRule(_ context.Context, r domain.OverrideRule) (*domain.OverrideRule, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	r.ID = 99
	return &r, nil
}
func (f *fakeOverrideService) GetOverrideRule(_ context.Context, _ int64) (*domain.OverrideRule, error) {
	return nil, nil
}
func (f *fakeOverrideService) UpdateOverrideRule(_ context.Context, r domain.OverrideRule) (*domain.OverrideRule, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &r, nil
}
func (f *fakeOverrideService) DeleteOverrideRule(_ context.Context, _ int64) error {
	return f.deleteErr
}
func (f *fakeOverrideService) ReorderOverrideRules(_ context.Context, _ []int64) error {
	return nil
}
func (f *fakeOverrideService) ListAreas(_ context.Context) ([]*domain.Area, error) {
	return f.areas, nil
}

// Remaining kitchenService stubs.
func (f *fakeOverrideService) CreateArea(_ context.Context, _ string) (*domain.Area, error) {
	return nil, nil
}
func (f *fakeOverrideService) ListAreasWithItems(_ context.Context) ([]*service.AreaSummary, error) {
	return nil, nil
}
func (f *fakeOverrideService) GetArea(_ context.Context, _ int64) (*domain.Area, error) {
	return nil, nil
}
func (f *fakeOverrideService) GetAreaWithItems(_ context.Context, _ int64) (*domain.Area, []*domain.Item, *domain.Photo, error) {
	return nil, nil, nil, nil
}
func (f *fakeOverrideService) UpdateArea(_ context.Context, _ int64, _ string) (*domain.Area, error) {
	return nil, nil
}
func (f *fakeOverrideService) DeleteArea(_ context.Context, _ int64) error       { return nil }
func (f *fakeOverrideService) DeletePhoto(_ context.Context, _ int64) error      { return nil }
func (f *fakeOverrideService) UploadPhoto(_ context.Context, _ int64, _ []byte, _ string) (*domain.Photo, []*domain.Item, error) {
	return nil, nil, nil
}
func (f *fakeOverrideService) CreateItem(_ context.Context, _ int64, _, _ string) (*domain.Item, error) {
	return nil, nil
}
func (f *fakeOverrideService) UpdateItem(_ context.Context, _ int64, _, _ string) (*domain.Item, error) {
	return nil, nil
}
func (f *fakeOverrideService) DeleteItem(_ context.Context, _ int64) error   { return nil }
func (f *fakeOverrideService) ReorderAreas(_ context.Context, _ []int64) error { return nil }
func (f *fakeOverrideService) SearchItems(_ context.Context, _ string) ([]*domain.Item, error) {
	return nil, nil
}
func (f *fakeOverrideService) ListSnapshots(_ context.Context, _ int64) ([]*domain.Snapshot, error) {
	return nil, nil
}

func newOverrideTestServer(svc kitchenService) *Server {
	return NewServer(svc, templates.FS, nil, slog.Default())
}

func TestHandleListOverrides_OK(t *testing.T) {
	svc := &fakeOverrideService{}
	srv := newOverrideTestServer(svc)

	req := httptest.NewRequest("GET", "/overrides", nil)
	rec := httptest.NewRecorder()
	srv.handleListOverrides(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Override Rules")
}

func TestHandleCreateOverride_OK(t *testing.T) {
	svc := &fakeOverrideService{}
	srv := newOverrideTestServer(svc)

	form := url.Values{
		"match_pattern": {"Milk"},
		"replacement":   {"Whole Milk"},
		"match_exact":   {"on"},
		"scope":         {"global"},
	}
	req := httptest.NewRequest("POST", "/overrides", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.handleCreateOverride(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/overrides", rec.Header().Get("Location"))
}

func TestHandleCreateOverride_MissingPattern(t *testing.T) {
	svc := &fakeOverrideService{}
	srv := newOverrideTestServer(svc)

	form := url.Values{
		"match_pattern": {""},
		"match_exact":   {"on"},
		"scope":         {"global"},
	}
	req := httptest.NewRequest("POST", "/overrides", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.handleCreateOverride(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCreateOverride_NoMatchMode(t *testing.T) {
	svc := &fakeOverrideService{}
	srv := newOverrideTestServer(svc)

	form := url.Values{
		"match_pattern": {"Milk"},
		"scope":         {"global"},
		// no match_exact or match_substring
	}
	req := httptest.NewRequest("POST", "/overrides", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.handleCreateOverride(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCreateOverride_ServiceError(t *testing.T) {
	svc := &fakeOverrideService{createErr: errors.New("db error")}
	srv := newOverrideTestServer(svc)

	form := url.Values{
		"match_pattern": {"Milk"},
		"match_exact":   {"on"},
		"scope":         {"global"},
	}
	req := httptest.NewRequest("POST", "/overrides", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.handleCreateOverride(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleDeleteOverride_OK(t *testing.T) {
	svc := &fakeOverrideService{}
	srv := newOverrideTestServer(svc)

	req := httptest.NewRequest("DELETE", "/overrides/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	srv.handleDeleteOverride(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleDeleteOverride_InvalidID(t *testing.T) {
	svc := &fakeOverrideService{}
	srv := newOverrideTestServer(svc)

	req := httptest.NewRequest("DELETE", "/overrides/notanid", nil)
	req.SetPathValue("id", "notanid")
	rec := httptest.NewRecorder()
	srv.handleDeleteOverride(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleUpdateOverride_OK(t *testing.T) {
	svc := &fakeOverrideService{}
	srv := newOverrideTestServer(svc)

	form := url.Values{
		"match_pattern": {"Milk"},
		"replacement":   {"Whole Milk"},
		"match_exact":   {"on"},
		"scope":         {"global"},
	}
	req := httptest.NewRequest("PUT", "/overrides/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	srv.handleUpdateOverride(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
}

// Ensure handleListOverrides writes full HTML (not empty body).
func TestHandleListOverrides_RendersHTML(t *testing.T) {
	svc := &fakeOverrideService{
		rules: []*domain.OverrideRule{
			{ID: 1, MatchPattern: "oj", Replacement: "Orange Juice", MatchExact: true, Scope: "global"},
		},
	}
	srv := newOverrideTestServer(svc)

	req := httptest.NewRequest("GET", "/overrides", nil)
	rec := httptest.NewRecorder()
	srv.handleListOverrides(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	assert.Contains(t, string(body), "oj")
	assert.Contains(t, string(body), "Orange Juice")
}

