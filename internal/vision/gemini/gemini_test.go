package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbonduro/kitchinv/internal/vision"
)

func makeServer(t *testing.T, respBody interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(respBody); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
}

func geminiResp(text string) map[string]interface{} {
	return map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{"text": text},
					},
				},
			},
		},
	}
}

func TestGeminiAnalyze(t *testing.T) {
	server := makeServer(t, geminiResp(`{"status":"ok","items":[{"name":"Milk","quantity":2,"notes":"top shelf"},{"name":"Eggs","quantity":12,"notes":"middle shelf"}]}`))
	defer server.Close()

	analyzer := NewGeminiAnalyzer("test-key", "gemini-2.0-flash")
	analyzer.baseURL = server.URL

	result, err := analyzer.Analyze(context.Background(), bytes.NewReader([]byte{0xFF, 0xD8}), "image/jpeg")
	require.NoError(t, err)
	assert.Equal(t, vision.StatusOK, result.Status)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "Milk", result.Items[0].Name)
	assert.Equal(t, "2", result.Items[0].Quantity)
	assert.Equal(t, "top shelf", result.Items[0].Notes)
	assert.Equal(t, "Eggs", result.Items[1].Name)
}

func TestGeminiAnalyzeNoItems(t *testing.T) {
	server := makeServer(t, geminiResp(`{"status":"no_items","items":[]}`))
	defer server.Close()

	analyzer := NewGeminiAnalyzer("test-key", "gemini-2.0-flash")
	analyzer.baseURL = server.URL

	result, err := analyzer.Analyze(context.Background(), bytes.NewReader([]byte{0xFF, 0xD8}), "image/jpeg")
	require.NoError(t, err)
	assert.Equal(t, vision.StatusNoItems, result.Status)
	assert.Empty(t, result.Items)
}

func TestGeminiAnalyzeUnclear(t *testing.T) {
	server := makeServer(t, geminiResp(`{"status":"unclear","items":[]}`))
	defer server.Close()

	analyzer := NewGeminiAnalyzer("test-key", "gemini-2.0-flash")
	analyzer.baseURL = server.URL

	_, err := analyzer.Analyze(context.Background(), bytes.NewReader([]byte{0xFF, 0xD8}), "image/jpeg")
	assert.Error(t, err)
}

func TestGeminiAnalyzeAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "quota exceeded", http.StatusTooManyRequests)
	}))
	defer server.Close()

	analyzer := NewGeminiAnalyzer("test-key", "gemini-2.0-flash")
	analyzer.baseURL = server.URL

	_, err := analyzer.Analyze(context.Background(), bytes.NewReader([]byte{0xFF, 0xD8}), "image/jpeg")
	assert.Error(t, err)
}

func TestGeminiAnalyzeReadError(t *testing.T) {
	analyzer := NewGeminiAnalyzer("test-key", "gemini-2.0-flash")

	_, err := analyzer.Analyze(context.Background(), &errReader{}, "image/jpeg")
	assert.Error(t, err)
}

func TestGeminiRequestIncludesAPIKey(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(geminiResp(`{"status":"ok","items":[]}`))
	}))
	defer server.Close()

	analyzer := NewGeminiAnalyzer("my-api-key", "gemini-2.0-flash")
	analyzer.baseURL = server.URL

	_, _ = analyzer.Analyze(context.Background(), bytes.NewReader([]byte{0xFF, 0xD8}), "image/jpeg")
	assert.Contains(t, capturedURL, "key=my-api-key")
}

func TestGeminiAnalyzeWithFileURI(t *testing.T) {
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(geminiResp(`{"status":"ok","items":[{"name":"Butter","quantity":1,"notes":"door shelf"}]}`))
	}))
	defer server.Close()

	analyzer := NewGeminiAnalyzer("test-key", "gemini-2.0-flash")
	analyzer.baseURL = server.URL

	result, err := analyzer.AnalyzeWithFileURI(context.Background(), "https://files.example.com/abc123", "image/jpeg")
	require.NoError(t, err)
	assert.Equal(t, vision.StatusOK, result.Status)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, "Butter", result.Items[0].Name)

	// Verify file_data (not inline_data) is in the request body
	assert.Contains(t, string(capturedBody), "file_data")
	assert.NotContains(t, string(capturedBody), "inline_data")
}

func TestGeminiAnalyzeWithFileURI_serverError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	analyzer := NewGeminiAnalyzer("test-key", "gemini-2.0-flash")
	analyzer.baseURL = server.URL

	_, err := analyzer.AnalyzeWithFileURI(context.Background(), "https://files.example.com/abc123", "image/jpeg")
	assert.Error(t, err)
}

// errReader always returns an error on Read.
type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
