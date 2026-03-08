package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// UploadFile uploads a local image file to the Gemini File API using the
// multipart upload protocol and returns the file URI. The URI can be reused
// across multiple generateContent calls for 48h, avoiding repeated base64
// encoding of large images.
//
// This is intended for benchmark use only — not used in the production path.
func UploadFile(ctx context.Context, apiKey, baseURL, filePath, mimeType string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %q: %w", filePath, err)
	}

	// Build the two-part body manually with a fixed boundary.
	// Gemini's multipart/related upload expects:
	//   Part 1: application/json metadata
	//   Part 2: raw image bytes with the image mime type
	const boundary = "boundary_kitchinv"

	metaJSON, err := json.Marshal(map[string]interface{}{
		"file": map[string]string{
			"display_name": filepath.Base(filePath),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var buf bytes.Buffer
	// Part 1: metadata
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: application/json; charset=UTF-8\r\n\r\n")
	buf.Write(metaJSON)
	fmt.Fprintf(&buf, "\r\n")
	// Part 2: image bytes
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: %s\r\n\r\n", mimeType)
	buf.Write(data)
	fmt.Fprintf(&buf, "\r\n--%s--\r\n", boundary)

	url := fmt.Sprintf("%s/upload/v1beta/files?key=%s", baseURL, apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "multipart/related; boundary="+boundary)
	req.Header.Set("X-Goog-Upload-Protocol", "multipart")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("file upload returned status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		File struct {
			URI string `json:"uri"`
		} `json:"file"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode upload response: %w", err)
	}

	return result.File.URI, nil
}
