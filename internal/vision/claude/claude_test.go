package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func TestClaudeAnalyzeStream(t *testing.T) {
	// Anthropic streaming SSE: each event is "event: <type>\ndata: <json>\n\n"
	events := []string{
		"event: message_start\ndata: {\"type\":\"message_start\"}\n\n",
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Milk | 1 liter | opened\"}}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"\\nButter | 250 g |\"}}\n\n",
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\"}\n\n",
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify stream:true was sent
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, true, req["stream"])

		w.Header().Set("Content-Type", "text/event-stream")
		for _, ev := range events {
			_, _ = w.Write([]byte(ev))
		}
	}))
	defer server.Close()

	analyzer := NewClaudeAnalyzer("sk-test", "claude-opus-4-6")
	analyzer.baseURL = server.URL

	ch, err := analyzer.AnalyzeStream(context.Background(), bytes.NewReader([]byte{0xFF, 0xD8}), "image/jpeg")
	require.NoError(t, err)

	var items []string
	for ev := range ch {
		require.NoError(t, ev.Err)
		items = append(items, ev.Item.Name)
	}

	assert.Equal(t, []string{"Milk", "Butter"}, items)
}

func TestClaudeAnalyzeStreamAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	analyzer := NewClaudeAnalyzer("sk-test", "claude-opus-4-6")
	analyzer.baseURL = server.URL

	_, err := analyzer.AnalyzeStream(context.Background(), bytes.NewReader([]byte{0xFF, 0xD8}), "image/jpeg")
	assert.Error(t, err)
}

func TestClaudeAnalyzeStreamContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 100; i++ {
			_, _ = fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"token\"}}\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	analyzer := NewClaudeAnalyzer("sk-test", "claude-opus-4-6")
	analyzer.baseURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := analyzer.AnalyzeStream(ctx, bytes.NewReader([]byte{0xFF, 0xD8}), "image/jpeg")
	require.NoError(t, err)

	cancel()

	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-context.Background().Done():
		t.Fatal("channel did not close after context cancel")
	}
}

// errReader always returns an error on Read.
type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
