package web

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
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
	defer closeWithLog(file, "upload file", s.logger)

	imageData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		s.logger.Error("read upload failed", "area_id", areaID, "error", err)
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
		s.logger.Error("upload photo failed", "area_id", areaID, "error", err)
		return
	}

	if err := s.renderPartial(w, "partials/item_list.html", items); err != nil {
		s.logger.Error("render partial failed", "error", err)
	}
}

// handleStreamPhoto handles the streaming upload flow. It accepts the same
// multipart form as handleUploadPhoto but responds with an SSE stream. Each
// SSE event carries a JSON object: {"name":"...","quantity":"...","notes":"..."}.
// The stream ends with a "done" event.
func (s *Server) handleStreamPhoto(w http.ResponseWriter, r *http.Request) {
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
	defer closeWithLog(file, "upload file", s.logger)

	imageData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		s.logger.Error("read upload failed", "area_id", areaID, "error", err)
		return
	}

	mimeType, ok := allowedImageMIME(imageData)
	if !ok {
		http.Error(w, "unsupported image format", http.StatusBadRequest)
		return
	}

	// Use a detached context so that the analysis runs to completion even if
	// the client navigates away and the request context is cancelled.
	_, itemCh, err := s.service.UploadPhotoStream(context.WithoutCancel(r.Context()), areaID, imageData, mimeType)
	if err != nil {
		http.Error(w, "failed to process photo", http.StatusInternalServerError)
		s.logger.Error("upload photo stream failed", "area_id", areaID, "error", err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, canFlush := w.(http.Flusher)

	enc := json.NewEncoder(w)
	for ev := range itemCh {
		if r.Context().Err() != nil {
			return
		}
		if ev.Err != nil {
			s.logger.Error("stream vision error", "area_id", areaID, "error", ev.Err)
			return
		}
		if _, err := w.Write([]byte("data: ")); err != nil {
			return
		}
		if err := enc.Encode(map[string]string{
			"name":     ev.Item.Name,
			"quantity": ev.Item.Quantity,
			"notes":    ev.Item.Notes,
		}); err != nil {
			return
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return
		}
		if canFlush {
			flusher.Flush()
		}
	}

	if _, err := w.Write([]byte("event: done\ndata: {}\n\n")); err != nil {
		s.logger.Error("write done event failed", "area_id", areaID, "error", err)
	}
	if canFlush {
		flusher.Flush()
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
		s.logger.Error("get area for photo failed", "area_id", areaID, "error", err)
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
	defer closeWithLog(reader, "photo reader", s.logger)

	w.Header().Set("Content-Type", mimeType)
	if _, err := io.Copy(w, reader); err != nil {
		s.logger.Error("write photo failed", "area_id", areaID, "error", err)
	}
}

// closeWithLog closes c and logs any error, using label to identify the resource.
func closeWithLog(c io.Closer, label string, logger *slog.Logger) {
	if err := c.Close(); err != nil {
		logger.Error("failed to close resource", "label", label, "error", err)
	}
}
