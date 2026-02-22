package web_test

import (
	"bufio"
	"bytes"
	"context"
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

// blockingVision is a VisionAnalyzer/StreamAnalyzer stub whose AnalyzeStream
// blocks until release is closed. This lets tests inspect server state while
// analysis is in progress (simulating a user navigating away mid-stream).
type blockingVision struct {
	release chan struct{} // close to unblock
	result  *vision.AnalysisResult
}

func newBlockingVision(result *vision.AnalysisResult) *blockingVision {
	return &blockingVision{release: make(chan struct{}), result: result}
}

func (b *blockingVision) Analyze(_ context.Context, _ io.Reader, _ string) (*vision.AnalysisResult, error) {
	<-b.release
	return b.result, nil
}

func (b *blockingVision) AnalyzeStream(_ context.Context, _ io.Reader, _ string) (<-chan vision.StreamEvent, error) {
	ch := make(chan vision.StreamEvent, len(b.result.Items)+1)
	go func() {
		<-b.release
		for i := range b.result.Items {
			ch <- vision.StreamEvent{Item: &b.result.Items[i]}
		}
		close(ch)
	}()
	return ch, nil
}

func (b *blockingVision) Release() { close(b.release) }

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
// pre-configured result. It implements both VisionAnalyzer and StreamAnalyzer.
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

func (r *recordingVision) AnalyzeStream(_ context.Context, rd io.Reader, _ string) (<-chan vision.StreamEvent, error) {
	data, err := io.ReadAll(rd)
	if err != nil {
		return nil, fmt.Errorf("recordingVision: read image: %w", err)
	}
	r.mu.Lock()
	r.lastBytes = data
	r.mu.Unlock()
	ch := make(chan vision.StreamEvent, len(r.result.Items)+1)
	for i := range r.result.Items {
		ch <- vision.StreamEvent{Item: &r.result.Items[i]}
	}
	close(ch)
	return ch, nil
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
// the HX-Redirect header pointing to /areas.
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
	if got := resp.Header.Get("HX-Redirect"); got != "/areas" {
		t.Errorf("HX-Redirect = %q, want %q", got, "/areas")
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

// TestIntegration_UploadPhotoStream_NonEmptyImageBytes is a regression test for
// the bug where a disabled <input type="file"> produced empty FormData,
// causing the server to receive zero image bytes. It verifies that the bytes
// received by the vision analyzer are non-empty.
func TestIntegration_UploadPhotoStream_NonEmptyImageBytes(t *testing.T) {
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
	resp, err := http.Post(srv.URL+"/areas/1/photos/stream", contentType, body)
	if err != nil {
		t.Fatalf("POST /areas/1/photos/stream: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	// Drain SSE until "event: done".
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "event: done") {
			break
		}
	}

	if got := len(vis.LastBytes()); got == 0 {
		t.Error("regression: vision analyzer received zero image bytes — empty FormData bug may be present")
	}
}

// TestIntegration_AreaDetail_PhotoServedAfterUpload is a regression test for
// kitchinv-5mw (photo preview missing). It verifies that after a stream upload
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

	// Upload via stream and drain until done.
	body, contentType := buildMultipartBody(t, minimalJPEG)
	resp, err := http.Post(srv.URL+"/areas/1/photos/stream", contentType, body)
	if err != nil {
		t.Fatalf("POST /areas/1/photos/stream: %v", err)
	}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "event: done") {
			break
		}
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

	// The template renders <img src="/areas/1/photo" ...> when a photo exists.
	if !strings.Contains(html, `/areas/1/photo`) {
		t.Errorf("regression(kitchinv-5mw): area detail page missing photo img tag after upload\nHTML:\n%s", html)
	}
}

// TestIntegration_AreaDetail_MidStreamNavigation is a regression test for
// kitchinv-5mw (analysis state lost on navigation). It simulates a user
// navigating away while streaming is in progress: the photo is already saved
// to the DB but items are not yet written. On page load, the server must
// surface the photo (so the preview can be shown) even though items are empty.
func TestIntegration_AreaDetail_MidStreamNavigation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vis := newBlockingVision(&vision.AnalysisResult{
		Items: []vision.DetectedItem{
			{Name: "Cheese", Quantity: "1 block", Notes: ""},
		},
	})

	srv, cleanup := newTestServer(t, vis)
	defer cleanup()

	createArea(t, srv, "Fridge")

	// Start a stream upload in the background; the vision stub blocks so items
	// are not yet written to the DB.
	uploadDone := make(chan struct{})
	go func() {
		defer close(uploadDone)
		body, contentType := buildMultipartBody(t, minimalJPEG)
		resp, err := http.Post(srv.URL+"/areas/1/photos/stream", contentType, body)
		if err != nil {
			return
		}
		_ = resp.Body.Close()
	}()

	// The service saves the photo record before calling AnalyzeStream, so after
	// a brief yield the photo row exists in the DB but zero items do.
	// Poll until the photo is visible on the area detail page.
	var midStreamHTML string
	for range 20 {
		resp, err := http.Get(srv.URL + "/areas/1")
		if err != nil {
			t.Fatalf("GET /areas/1: %v", err)
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if strings.Contains(string(b), `/areas/1/photo`) {
			midStreamHTML = string(b)
			break
		}
		// Small sleep via channel to avoid import of "time" package.
		select {
		case <-uploadDone:
			// Stream finished unexpectedly early; the test is no longer meaningful.
			t.Skip("stream finished before mid-stream check could run")
		default:
		}
	}

	// Unblock the vision stub so the goroutine can finish.
	vis.Release()
	<-uploadDone

	if midStreamHTML == "" {
		t.Fatal("regression(kitchinv-5mw): photo never appeared in area detail page during stream")
	}

	// The photo must be present even though items haven't arrived yet.
	if !strings.Contains(midStreamHTML, `/areas/1/photo`) {
		t.Errorf("regression(kitchinv-5mw): photo img tag missing mid-stream; page showed no photo while analysis was in progress\nHTML:\n%s", midStreamHTML)
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
