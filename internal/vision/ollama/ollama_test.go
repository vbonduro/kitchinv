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
