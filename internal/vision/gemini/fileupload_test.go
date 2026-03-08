package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadFile_success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"file": map[string]interface{}{
				"uri": "https://generativelanguage.googleapis.com/v1beta/files/abc123",
			},
		})
	}))
	defer server.Close()

	uri, err := UploadFile(context.Background(), "test-key", server.URL, "testdata/test.jpg", "image/jpeg")
	require.NoError(t, err)
	assert.Equal(t, "https://generativelanguage.googleapis.com/v1beta/files/abc123", uri)
}

func TestUploadFile_apiKeyInURL(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"file": map[string]interface{}{
				"uri": "https://example.com/files/xyz",
			},
		})
	}))
	defer server.Close()

	_, err := UploadFile(context.Background(), "my-special-key", server.URL, "testdata/test.jpg", "image/jpeg")
	require.NoError(t, err)
	assert.Contains(t, capturedURL, "key=my-special-key")
}

func TestUploadFile_serverError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := UploadFile(context.Background(), "test-key", server.URL, "testdata/test.jpg", "image/jpeg")
	assert.Error(t, err)
}
