package web

import (
	"io"
	"log"
	"net/http"
)

const maxPhotoSize = 50 * 1024 * 1024 // 50 MB

// allowedImageTypes is the set of MIME types accepted for uploaded photos.
// net/http.DetectContentType handles JPEG, PNG, and GIF via magic-byte
// sniffing. WebP is detected separately because the WHATWG sniff spec (and
// therefore the stdlib) does not include a WebP signature.
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
}

// isWebP reports whether data is a WebP image (RIFF container with "WEBP" at
// offset 8).
func isWebP(data []byte) bool {
	return len(data) >= 12 &&
		string(data[0:4]) == "RIFF" &&
		string(data[8:12]) == "WEBP"
}

// allowedImageMIME returns the detected MIME type and true if the data is an
// accepted image format, or ("", false) otherwise.
func allowedImageMIME(data []byte) (string, bool) {
	if isWebP(data) {
		return "image/webp", true
	}
	mime := http.DetectContentType(data)
	if allowedImageTypes[mime] {
		return mime, true
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

	mimeType, ok := allowedImageMIME(imageData)
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
