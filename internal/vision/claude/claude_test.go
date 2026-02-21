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
)

func TestClaudeAnalyze(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "Milk | 1 liter | opened\nButter | 250 g |"},
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
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "Milk", result.Items[0].Name)
	assert.Equal(t, "1 liter", result.Items[0].Quantity)
	assert.Equal(t, "opened", result.Items[0].Notes)
	assert.Equal(t, "Butter", result.Items[1].Name)
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

	// A LimitedReader with N=0 returns EOF immediately, so ReadAll returns empty — not an error.
	// Use an errReader that returns an error on Read instead.
	_, err := analyzer.Analyze(context.Background(), &errReader{}, "image/jpeg")
	assert.Error(t, err)
}

// errReader always returns an error on Read.
type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
