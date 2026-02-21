package web

import (
	"bytes"
	"io"
	"log"
	"net/http"
)

const maxPhotoSize = 50 * 1024 * 1024 // 50 MB

// allowedMagicBytes maps the first bytes of each supported image format to its
// canonical MIME type. We detect type from content, not from the client header.
var allowedMagicBytes = []struct {
	magic    []byte
	mimeType string
}{
	{[]byte{0xFF, 0xD8, 0xFF}, "image/jpeg"},
	{[]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, "image/png"},
	{[]byte{0x47, 0x49, 0x46, 0x38}, "image/gif"},
	{[]byte("RIFF"), "image/webp"}, // full check also validates "WEBP" at offset 8
}

// detectImageMIME returns the canonical MIME type by inspecting magic bytes.
// It returns ("", false) if the data does not match a supported image format.
func detectImageMIME(data []byte) (string, bool) {
	for _, entry := range allowedMagicBytes {
		if bytes.HasPrefix(data, entry.magic) {
			// Extra check for WebP: bytes 8-11 must be "WEBP"
			if entry.mimeType == "image/webp" {
				if len(data) < 12 || string(data[8:12]) != "WEBP" {
					continue
				}
			}
			return entry.mimeType, true
		}
	}
	return "", false
}

func (s *Server) handleUploadPhoto(w http.ResponseWriter, r *http.Request) {
	areaID, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid area id", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(maxPhotoSize); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "image file required", http.StatusBadRequest)
		return
	}
	defer closeWithLog(file, "upload file")

	imageData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		log.Printf("read upload error: %v", err)
		return
	}

	mimeType, ok := detectImageMIME(imageData)
	if !ok {
		http.Error(w, "unsupported image format", http.StatusBadRequest)
		return
	}

	_, items, err := s.service.UploadPhoto(r.Context(), areaID, imageData, mimeType)
	if err != nil {
		http.Error(w, "failed to process photo", http.StatusInternalServerError)
		log.Printf("upload photo error: %v", err)
		return
	}

	if err := s.renderPartial(w, "partials/item_list.html", items); err != nil {
		log.Printf("render partial error: %v", err)
	}
}

func (s *Server) handleGetPhoto(w http.ResponseWriter, r *http.Request) {
	areaID, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid area id", http.StatusBadRequest)
		return
	}

	_, _, photo, err := s.service.GetAreaWithItems(r.Context(), areaID)
	if err != nil {
		http.Error(w, "failed to get area", http.StatusInternalServerError)
		log.Printf("get area for photo error: %v", err)
		return
	}
	if photo == nil {
		http.NotFound(w, r)
		return
	}

	reader, mimeType, err := s.photoStore.Get(r.Context(), photo.StorageKey)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer closeWithLog(reader, "photo reader")

	w.Header().Set("Content-Type", mimeType)
	if _, err := io.Copy(w, reader); err != nil {
		log.Printf("write photo error: %v", err)
	}
}

// closeWithLog closes c and logs any error, using label to identify the resource.
func closeWithLog(c io.Closer, label string) {
	if err := c.Close(); err != nil {
		log.Printf("failed to close %s: %v", label, err)
	}
}
