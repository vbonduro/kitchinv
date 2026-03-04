package claude

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

func TestClaudeAnalyze(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify system prompt is included in the request.
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.NotEmpty(t, req["system"], "system prompt should be set")

		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"status":"ok","items":[{"name":"Milk","quantity":1,"notes":"opened"},{"name":"Butter","quantity":1,"notes":null}]}`},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	analyzer := NewClaudeAnalyzer("sk-test", "claude-opus-4-6")
	analyzer.baseURL = server.URL

	result, err := analyzer.Analyze(context.Background(), bytes.NewReader([]byte{0xFF, 0xD8}), "image/jpeg")
	require.NoError(t, err)
	assert.Equal(t, vision.StatusOK, result.Status)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "Milk", result.Items[0].Name)
	assert.Equal(t, "1", result.Items[0].Quantity)
	assert.Equal(t, "opened", result.Items[0].Notes)
	assert.Equal(t, "Butter", result.Items[1].Name)
}

func TestClaudeAnalyzeNoItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"status":"no_items","items":[]}`},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	analyzer := NewClaudeAnalyzer("sk-test", "claude-opus-4-6")
	analyzer.baseURL = server.URL

	result, err := analyzer.Analyze(context.Background(), bytes.NewReader([]byte{0xFF, 0xD8}), "image/jpeg")
	require.NoError(t, err)
	assert.Equal(t, vision.StatusNoItems, result.Status)
	assert.Empty(t, result.Items)
}

func TestClaudeAnalyzeUnclear(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"status":"unclear","items":[]}`},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	analyzer := NewClaudeAnalyzer("sk-test", "claude-opus-4-6")
	analyzer.baseURL = server.URL

	_, err := analyzer.Analyze(context.Background(), bytes.NewReader([]byte{0xFF, 0xD8}), "image/jpeg")
	// unclear is returned as an error so the caller can prompt the user to retake.
	assert.Error(t, err)
}

func TestClaudeAnalyzeAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	analyzer := NewClaudeAnalyzer("sk-test", "claude-opus-4-6")
	analyzer.baseURL = server.URL

	_, err := analyzer.Analyze(context.Background(), bytes.NewReader([]byte{0xFF, 0xD8}), "image/jpeg")
	assert.Error(t, err)
}

func TestClaudeAnalyzeReadError(t *testing.T) {
	analyzer := NewClaudeAnalyzer("sk-test", "claude-opus-4-6")

	_, err := analyzer.Analyze(context.Background(), &errReader{}, "image/jpeg")
	assert.Error(t, err)
}

// errReader always returns an error on Read.
type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
