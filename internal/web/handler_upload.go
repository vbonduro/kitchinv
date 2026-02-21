package web

import (
	"io"
	"log"
	"net/http"
)

const maxPhotoSize = 50 * 1024 * 1024 // 50 MB

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

	file, header, err := r.FormFile("image")
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

	_, items, err := s.service.UploadPhoto(r.Context(), areaID, imageData, header.Header.Get("Content-Type"))
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
