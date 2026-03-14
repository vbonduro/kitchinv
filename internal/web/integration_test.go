package web_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/vbonduro/kitchinv/internal/db"
	"github.com/vbonduro/kitchinv/internal/service"
	"github.com/vbonduro/kitchinv/internal/store"
	"github.com/vbonduro/kitchinv/internal/vision"
	"github.com/vbonduro/kitchinv/internal/web"
	"github.com/vbonduro/kitchinv/internal/web/templates"
)

// failingVision is a VisionAnalyzer stub that always returns an error.
type failingVision struct {
	err error
}

func (f *failingVision) Analyze(_ context.Context, _ io.Reader, _ string) (*vision.AnalysisResult, error) {
	return nil, f.err
}

// minimalJPEG is 512 bytes with the JPEG magic bytes header followed by zeros.
// http.DetectContentType identifies JPEG from the leading 0xFF 0xD8 bytes.
var minimalJPEG = func() []byte {
	b := make([]byte, 512)
	b[0] = 0xFF
	b[1] = 0xD8
	b[2] = 0xFF
	b[3] = 0xE0
	return b
}()

// recordingVision captures the image bytes passed to it and returns a
// pre-configured result.
type recordingVision struct {
	mu        sync.Mutex
	lastBytes []byte
	result    *vision.AnalysisResult
}

func (r *recordingVision) Analyze(_ context.Context, rd io.Reader, _ string) (*vision.AnalysisResult, error) {
	data, err := io.ReadAll(rd)
	if err != nil {
		return nil, fmt.Errorf("recordingVision: read image: %w", err)
	}
	r.mu.Lock()
	r.lastBytes = data
	r.mu.Unlock()
	return r.result, nil
}

func (r *recordingVision) LastBytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastBytes
}

// memPhotoStore is a simple in-memory implementation of photostore.PhotoStore.
type memPhotoStore struct {
	mu      sync.Mutex
	data    map[string][]byte
	mimes   map[string]string
	counter int
}

func newMemPhotoStore() *memPhotoStore {
	return &memPhotoStore{
		data:  make(map[string][]byte),
		mimes: make(map[string]string),
	}
}

func (m *memPhotoStore) Save(_ context.Context, prefix, mimeType string, r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counter++
	key := fmt.Sprintf("%s_%d", prefix, m.counter)
	m.data[key] = data
	m.mimes[key] = mimeType
	return key, nil
}

func (m *memPhotoStore) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.data[key]
	if !ok {
		return nil, "", fmt.Errorf("key not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(data)), m.mimes[key], nil
}

func (m *memPhotoStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	delete(m.mimes, key)
	return nil
}

// blockingVision signals ready when Analyze is called, then blocks until
// release is closed, allowing tests to inspect intermediate server state.
type blockingVision struct {
	ready   chan struct{}
	release chan struct{}
	result  *vision.AnalysisResult
}

func (b *blockingVision) Analyze(_ context.Context, _ io.Reader, _ string) (*vision.AnalysisResult, error) {
	close(b.ready)
	<-b.release
	return b.result, nil
}

// newTestServer sets up a real web.Server backed by in-memory SQLite and the
// provided vision stub. Returns the test server and a cleanup function.
func newTestServer(t *testing.T, vis vision.VisionAnalyzer) (*httptest.Server, func()) {
	t.Helper()
	database, err := db.OpenForTesting()
	if err != nil {
		t.Fatalf("OpenForTesting: %v", err)
	}

	svc := service.NewAreaService(
		store.NewAreaStore(database),
		store.NewPhotoStore(database),
		store.NewItemStore(database),
		store.NewItemEditStore(database),
		vis,
		newMemPhotoStore(),
		slog.Default(),
	)
	srv := httptest.NewServer(web.NewServer(svc, templates.FS, newMemPhotoStore(), slog.Default()))
	return srv, func() {
		srv.Close()
		_ = database.Close()
	}
}

// createArea posts to /areas and returns the area ID.
// Each test uses a fresh in-memory SQLite database so IDs are sequential
// starting at 1; the n-th call to createArea within a test returns n.
func createArea(t *testing.T, srv *httptest.Server, name string) int64 {
	t.Helper()
	resp, err := http.PostForm(srv.URL+"/areas", url.Values{"name": {name}})
	if err != nil {
		t.Fatalf("POST /areas: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /areas status %d: %s", resp.StatusCode, body)
	}
	// SQLite auto-increment starts at 1 in a fresh DB; the first area is ID=1.
	// Tests that need multiple areas track the count themselves.
	return 1
}

// buildMultipartBody creates a multipart/form-data body with an "image" field.
func buildMultipartBody(t *testing.T, imageData []byte) (body *bytes.Buffer, contentType string) {
	t.Helper()
	body = &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, err := w.CreateFormFile("image", "photo.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(imageData); err != nil {
		t.Fatalf("write image data: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body, w.FormDataContentType()
}

// TestIntegration_CreateArea verifies that POST /areas with a name succeeds.
func TestIntegration_CreateArea(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vis := &recordingVision{result: &vision.AnalysisResult{}}
	srv, cleanup := newTestServer(t, vis)
	defer cleanup()

	resp, err := http.PostForm(srv.URL+"/areas", url.Values{"name": {"Fridge"}})
	if err != nil {
		t.Fatalf("POST /areas: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "Fridge") {
		t.Errorf("response body does not contain 'Fridge':\n%s", body)
	}
}

// TestIntegration_ListAreas verifies that GET /areas returns 200 after creating an area.
func TestIntegration_ListAreas(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vis := &recordingVision{result: &vision.AnalysisResult{}}
	srv, cleanup := newTestServer(t, vis)
	defer cleanup()

	// Create an area first.
	resp, err := http.PostForm(srv.URL+"/areas", url.Values{"name": {"Pantry"}})
	if err != nil {
		t.Fatalf("POST /areas: %v", err)
	}
	_ = resp.Body.Close()

	// Now list areas.
	resp, err = http.Get(srv.URL + "/areas")
	if err != nil {
		t.Fatalf("GET /areas: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

// TestIntegration_DeleteArea verifies that DELETE /areas/{id} returns 200 with
// an empty body (HTMX removes the card from the DOM).
func TestIntegration_DeleteArea(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vis := &recordingVision{result: &vision.AnalysisResult{}}
	srv, cleanup := newTestServer(t, vis)
	defer cleanup()

	// Create area (ID will be 1 in fresh DB).
	createArea(t, srv, "Garage")

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/areas/1", nil)
	if err != nil {
		t.Fatalf("new DELETE request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /areas/1: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Errorf("expected empty body, got %q", body)
	}
}

// TestIntegration_UploadPhoto verifies that uploading a valid JPEG returns 200
// and the response body contains the item name returned by the stub vision.
func TestIntegration_UploadPhoto(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vis := &recordingVision{
		result: &vision.AnalysisResult{
			Items: []vision.DetectedItem{
				{Name: "Orange Juice", Quantity: "1 carton", Notes: ""},
			},
		},
	}
	srv, cleanup := newTestServer(t, vis)
	defer cleanup()

	createArea(t, srv, "Fridge")

	body, contentType := buildMultipartBody(t, minimalJPEG)
	resp, err := http.Post(srv.URL+"/areas/1/photos", contentType, body)
	if err != nil {
		t.Fatalf("POST /areas/1/photos: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(b), "Orange Juice") {
		t.Errorf("response body does not contain 'Orange Juice':\n%s", b)
	}
}

// TestIntegration_UploadPhoto_NonEmptyImageBytes is a regression test for
// the bug where a disabled <input type="file"> produced empty FormData,
// causing the server to receive zero image bytes. It verifies that the bytes
// received by the vision analyzer are non-empty.
func TestIntegration_UploadPhoto_NonEmptyImageBytes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vis := &recordingVision{
		result: &vision.AnalysisResult{
			Items: []vision.DetectedItem{
				{Name: "Butter", Quantity: "1 pack", Notes: ""},
			},
		},
	}
	srv, cleanup := newTestServer(t, vis)
	defer cleanup()

	createArea(t, srv, "Fridge")

	body, contentType := buildMultipartBody(t, minimalJPEG)
	resp, err := http.Post(srv.URL+"/areas/1/photos", contentType, body)
	if err != nil {
		t.Fatalf("POST /areas/1/photos: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	if got := len(vis.LastBytes()); got == 0 {
		t.Error("regression: vision analyzer received zero image bytes — empty FormData bug may be present")
	}
}

// TestIntegration_AreaDetail_PhotoServedAfterUpload is a regression test for
// kitchinv-5mw (photo preview missing). It verifies that after an upload
// completes, GET /areas/{id} includes an <img> tag pointing to the photo
// endpoint, confirming the server correctly exposes the photo for the preview.
func TestIntegration_AreaDetail_PhotoServedAfterUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vis := &recordingVision{
		result: &vision.AnalysisResult{
			Items: []vision.DetectedItem{
				{Name: "Apple", Quantity: "3", Notes: ""},
			},
		},
	}
	srv, cleanup := newTestServer(t, vis)
	defer cleanup()

	createArea(t, srv, "Fridge")

	body, contentType := buildMultipartBody(t, minimalJPEG)
	resp, err := http.Post(srv.URL+"/areas/1/photos", contentType, body)
	if err != nil {
		t.Fatalf("POST /areas/1/photos: %v", err)
	}
	_ = resp.Body.Close()

	// Now load the area detail page and assert a photo <img> is rendered.
	resp, err = http.Get(srv.URL + "/areas/1")
	if err != nil {
		t.Fatalf("GET /areas/1: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	html := string(b)

	if !strings.Contains(html, `/areas/1/photo`) {
		t.Errorf("regression(kitchinv-5mw): area detail page missing photo img tag after upload\nHTML:\n%s", html)
	}
}

// TestIntegration_GetAreaItems verifies that GET /areas/{id}/items returns the
// item_list partial. This endpoint is used by the page-load polling fallback
// when a user navigates back to an area mid-stream (kitchinv-5mw).
func TestIntegration_GetAreaItems(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vis := &recordingVision{
		result: &vision.AnalysisResult{
			Items: []vision.DetectedItem{
				{Name: "Butter", Quantity: "1 pack", Notes: ""},
			},
		},
	}
	srv, cleanup := newTestServer(t, vis)
	defer cleanup()

	createArea(t, srv, "Fridge")

	// No items yet — endpoint should return empty state.
	resp, err := http.Get(srv.URL + "/areas/1/items")
	if err != nil {
		t.Fatalf("GET /areas/1/items: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	if strings.Contains(string(b), "item-row") {
		t.Errorf("expected no items before upload, got: %s", b)
	}

	// Upload so items are stored.
	body, contentType := buildMultipartBody(t, minimalJPEG)
	uploadResp, err := http.Post(srv.URL+"/areas/1/photos", contentType, body)
	if err != nil {
		t.Fatalf("POST /areas/1/photos: %v", err)
	}
	_ = uploadResp.Body.Close()

	// Now the items endpoint should include the detected item.
	resp, err = http.Get(srv.URL + "/areas/1/items")
	if err != nil {
		t.Fatalf("GET /areas/1/items: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	b, _ = io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "Butter") {
		t.Errorf("regression(kitchinv-5mw): items endpoint did not return stored items:\n%s", b)
	}
}

// TestIntegration_UploadPhoto_AnalysisFailure_NoExistingPhoto is a regression
// test for kitchinv-uh7. When the vision API rejects an upload on an area with
// no prior photo, the server must return a non-200 response.
func TestIntegration_UploadPhoto_AnalysisFailure_NoExistingPhoto(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vis := &failingVision{err: fmt.Errorf("image exceeds 5 MB maximum")}
	srv, cleanup := newTestServer(t, vis)
	defer cleanup()

	createArea(t, srv, "Fridge")

	body, contentType := buildMultipartBody(t, minimalJPEG)
	resp, err := http.Post(srv.URL+"/areas/1/photos", contentType, body)
	if err != nil {
		t.Fatalf("POST /areas/1/photos: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatal("regression(kitchinv-uh7): expected non-200 on vision failure, got 200")
	}
}


// TestIntegration_Search verifies that items stored after an upload are
// findable via GET /search?q=<term>.
func TestIntegration_Search(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vis := &recordingVision{
		result: &vision.AnalysisResult{
			Items: []vision.DetectedItem{
				{Name: "Milk", Quantity: "2 liters", Notes: ""},
			},
		},
	}
	srv, cleanup := newTestServer(t, vis)
	defer cleanup()

	createArea(t, srv, "Fridge")

	// Upload a photo so items are stored.
	body, contentType := buildMultipartBody(t, minimalJPEG)
	resp, err := http.Post(srv.URL+"/areas/1/photos", contentType, body)
	if err != nil {
		t.Fatalf("POST /areas/1/photos: %v", err)
	}
	_ = resp.Body.Close()

	// Search for the item.
	resp, err = http.Get(srv.URL + "/search?q=milk")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(b), "Milk") {
		t.Errorf("search response does not contain 'Milk':\n%s", b)
	}
}

// TestIntegration_RenameArea_DuplicateName verifies that PUT /areas/{id} with a
// name already used by another area returns 409 with a descriptive message.
func TestIntegration_GetAreaCard_NoPhoto(t *testing.T) {
	srv, cleanup := newTestServer(t, &failingVision{err: errors.New("unused")})
	t.Cleanup(cleanup)

	createArea(t, srv, "Pantry")

	resp, err := http.Get(srv.URL + "/areas/1/card")
	if err != nil {
		t.Fatalf("GET /areas/1/card: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	b, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	html := string(b)
	if !strings.Contains(html, "area-card") {
		t.Errorf("expected area-card in response, got: %s", html)
	}
	if !strings.Contains(html, "upload-zone") {
		t.Errorf("expected upload-zone (no photo state), got: %s", html)
	}
	if strings.Contains(html, "analyzing-indicator") {
		t.Errorf("unexpected analyzing-indicator with no photo: %s", html)
	}
}

func TestIntegration_GetAreaCard_AnalysingState_ShowsOverlay(t *testing.T) {
	// A slow vision stub that blocks until released, so we can inspect the
	// area card while the photo is committed but items not yet written.
	ready := make(chan struct{})
	release := make(chan struct{})
	slowVis := &blockingVision{
		ready:   ready,
		release: release,
		result: &vision.AnalysisResult{
			Items: []vision.DetectedItem{{Name: "Milk", Quantity: "1"}},
		},
	}

	srv, cleanup := newTestServer(t, slowVis)
	t.Cleanup(cleanup)

	createArea(t, srv, "Fridge")

	// Start upload in background — it will block in vision analysis.
	uploadDone := make(chan struct{})
	go func() {
		defer close(uploadDone)
		body, contentType := buildMultipartBody(t, minimalJPEG)
		resp, err := http.Post(srv.URL+"/areas/1/photos", contentType, body)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()

	// Wait until the photo is saved and vision analysis has started.
	<-ready

	// At this point the area has a photo but no items — analysing state.
	resp, err := http.Get(srv.URL + "/areas/1/card")
	if err != nil {
		t.Fatalf("GET /areas/1/card: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	html := string(b)

	if !strings.Contains(html, "analyzing-indicator-1") {
		t.Errorf("expected analyzing-indicator in analysing state, got: %s", html)
	}
	if strings.Contains(html, "upload-zone") {
		t.Errorf("unexpected upload-zone during analysis: %s", html)
	}

	// Release the vision stub and wait for upload to finish.
	close(release)
	<-uploadDone

	// After analysis completes, card should show items, no overlay.
	resp, err = http.Get(srv.URL + "/areas/1/card")
	if err != nil {
		t.Fatalf("GET /areas/1/card after complete: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	b, _ = io.ReadAll(resp.Body)
	html = string(b)

	if strings.Contains(html, "analyzing-indicator") {
		t.Errorf("unexpected analyzing-indicator after analysis complete: %s", html)
	}
	if !strings.Contains(html, "Milk") {
		t.Errorf("expected item 'Milk' after analysis complete, got: %s", html)
	}
}

func TestIntegration_RenameArea_DuplicateName(t *testing.T) {
	srv, cleanup := newTestServer(t, &failingVision{err: errors.New("unused")})
	t.Cleanup(cleanup)

	createArea(t, srv, "Fridge")
	createArea(t, srv, "Pantry")

	// Try to rename Pantry (id=2) to "Fridge".
	body := strings.NewReader(`{"name":"Fridge"}`)
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/areas/2", body)
	if err != nil {
		t.Fatalf("build PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /areas/2: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409, got %d: %s", resp.StatusCode, b)
	}

	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "already exists") {
		t.Errorf("expected 'already exists' in response body, got: %s", b)
	}
}

// TestIntegration_ReorderAreas verifies that POST /areas/reorder persists the
// new sort order and that a subsequent GET /areas returns cards in that order.
func TestIntegration_ReorderAreas(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vis := &recordingVision{result: &vision.AnalysisResult{}}
	srv, cleanup := newTestServer(t, vis)
	defer cleanup()

	// Create three areas: Alpha(1), Beta(2), Gamma(3).
	for _, name := range []string{"Alpha", "Beta", "Gamma"} {
		resp, err := http.PostForm(srv.URL+"/areas", url.Values{"name": {name}})
		if err != nil {
			t.Fatalf("POST /areas: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST /areas status %d", resp.StatusCode)
		}
	}

	// Reorder to Gamma(3), Alpha(1), Beta(2).
	reorderBody := strings.NewReader(`{"ids":[3,1,2]}`)
	resp, err := http.Post(srv.URL+"/areas/reorder", "application/json", reorderBody)
	if err != nil {
		t.Fatalf("POST /areas/reorder: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	// GET /areas and verify card order in the HTML.
	resp2, err := http.Get(srv.URL + "/areas")
	if err != nil {
		t.Fatalf("GET /areas: %v", err)
	}
	t.Cleanup(func() { _ = resp2.Body.Close() })
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	html, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	page := string(html)

	gammaPos := strings.Index(page, "Gamma")
	alphaPos := strings.Index(page, "Alpha")
	betaPos := strings.Index(page, "Beta")
	if gammaPos < 0 || alphaPos < 0 || betaPos < 0 {
		t.Fatalf("expected all area names in HTML, got:\n%s", page)
	}
	if gammaPos > alphaPos || alphaPos > betaPos {
		t.Errorf("expected order Gamma < Alpha < Beta in HTML, got positions: Gamma=%d Alpha=%d Beta=%d",
			gammaPos, alphaPos, betaPos)
	}
}

// TestIntegration_PhotoTimestamp verifies that after uploading a photo the area
// card includes a human-readable upload timestamp.
func TestIntegration_PhotoTimestamp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vis := &recordingVision{result: &vision.AnalysisResult{}}
	srv, cleanup := newTestServer(t, vis)
	defer cleanup()

	// Create area.
	resp, err := http.PostForm(srv.URL+"/areas", url.Values{"name": {"Fridge"}})
	if err != nil {
		t.Fatalf("POST /areas: %v", err)
	}
	_ = resp.Body.Close()

	// Upload a photo.
	body, ct := buildMultipartBody(t, minimalJPEG)
	resp2, err := http.Post(srv.URL+"/areas/1/photos", ct, body)
	if err != nil {
		t.Fatalf("POST photo: %v", err)
	}
	t.Cleanup(func() { _ = resp2.Body.Close() })
	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200, got %d: %s", resp2.StatusCode, b)
	}

	// GET /areas and verify a timestamp is present.
	resp3, err := http.Get(srv.URL + "/areas")
	if err != nil {
		t.Fatalf("GET /areas: %v", err)
	}
	t.Cleanup(func() { _ = resp3.Body.Close() })
	html, err := io.ReadAll(resp3.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if !strings.Contains(string(html), "photo-timestamp") {
		t.Errorf("expected photo-timestamp element in HTML, got:\n%s", html)
	}
}
