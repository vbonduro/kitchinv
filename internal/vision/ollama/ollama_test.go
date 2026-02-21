package ollama

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

func TestOllamaAnalyze(t *testing.T) {
	// Create a test server that mimics Ollama
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := map[string]interface{}{
			"model":    req.Model,
			"response": "Milk | 1 liter |\nButter | 1 block | opened",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	analyzer := NewOllamaAnalyzer(server.URL, "moondream")

	// Provide dummy image data
	imageData := []byte{0xFF, 0xD8, 0xFF, 0xE0} // JPEG header
	result, err := analyzer.Analyze(context.Background(), bytes.NewReader(imageData), "image/jpeg")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "Milk", result.Items[0].Name)
	assert.Equal(t, "1 liter", result.Items[0].Quantity)
	assert.Equal(t, "Butter", result.Items[1].Name)
	assert.Equal(t, "opened", result.Items[1].Notes)
}

func TestOllamaAnalyzeNetworkError(t *testing.T) {
	analyzer := NewOllamaAnalyzer("http://localhost:99999", "moondream")

	imageData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	_, err := analyzer.Analyze(context.Background(), bytes.NewReader(imageData), "image/jpeg")

	assert.Error(t, err)
}

func TestOllamaAnalyzeInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	analyzer := NewOllamaAnalyzer(server.URL, "moondream")

	imageData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	_, err := analyzer.Analyze(context.Background(), bytes.NewReader(imageData), "image/jpeg")

	assert.Error(t, err)
}

func TestOllamaAnalyzeReadError(t *testing.T) {
	analyzer := NewOllamaAnalyzer("http://localhost:11434", "moondream")

	// Create a reader that fails
	failReader := &io.LimitedReader{R: bytes.NewReader([]byte{0xFF}), N: 0}
	_, err := analyzer.Analyze(context.Background(), failReader, "image/jpeg")

	assert.Error(t, err)
}

func TestOllamaAnalyzeStream(t *testing.T) {
	// Ollama streaming sends one JSON object per line with stream:true
	chunks := []map[string]interface{}{
		{"response": "Milk | 1 liter | opened", "done": false},
		{"response": "\n", "done": false},
		{"response": "Butter | 1 block |", "done": false},
		{"response": "\n", "done": true},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify stream:true was sent
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, true, req["stream"])

		w.Header().Set("Content-Type", "application/x-ndjson")
		enc := json.NewEncoder(w)
		for _, chunk := range chunks {
			_ = enc.Encode(chunk)
		}
	}))
	defer server.Close()

	analyzer := NewOllamaAnalyzer(server.URL, "moondream")
	imageData := []byte{0xFF, 0xD8, 0xFF, 0xE0}

	ch, err := analyzer.AnalyzeStream(context.Background(), bytes.NewReader(imageData), "image/jpeg")
	require.NoError(t, err)

	var items []string
	for ev := range ch {
		require.NoError(t, ev.Err)
		items = append(items, ev.Item.Name)
	}

	assert.Equal(t, []string{"Milk", "Butter"}, items)
}

func TestOllamaAnalyzeStreamContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow server — never completes
		enc := json.NewEncoder(w)
		for i := 0; i < 100; i++ {
			_ = enc.Encode(map[string]interface{}{"response": "token", "done": false})
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	analyzer := NewOllamaAnalyzer(server.URL, "moondream")
	imageData := []byte{0xFF, 0xD8, 0xFF, 0xE0}

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := analyzer.AnalyzeStream(ctx, bytes.NewReader(imageData), "image/jpeg")
	require.NoError(t, err)

	// Cancel immediately; channel should close without hanging
	cancel()

	// Drain channel with timeout
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

func TestOllamaAnalyzeStreamHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	analyzer := NewOllamaAnalyzer(server.URL, "moondream")
	imageData := []byte{0xFF, 0xD8, 0xFF, 0xE0}

	_, err := analyzer.AnalyzeStream(context.Background(), bytes.NewReader(imageData), "image/jpeg")
	assert.Error(t, err)
}
