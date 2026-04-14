package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
)

func (s *Server) handleAPIDBHash(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open(s.dbPath)
	if err != nil {
		http.Error(w, "failed to open database", http.StatusInternalServerError)
		s.logger.Error("api db hash: open db", "error", err)
		return
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		http.Error(w, "failed to hash database", http.StatusInternalServerError)
		s.logger.Error("api db hash: hash db", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Hash string `json:"hash"`
	}{Hash: hex.EncodeToString(h.Sum(nil))})
}

func (s *Server) handleAPIDB(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.service.ListAreasWithItems(r.Context())
	if err != nil {
		http.Error(w, "failed to list areas", http.StatusInternalServerError)
		s.logger.Error("api db: list areas", "error", err)
		return
	}

	type itemResponse struct {
		Name     string `json:"Name"`
		Quantity string `json:"Quantity"`
	}
	type areaResponse struct {
		ID    int64          `json:"id"`
		Name  string         `json:"name"`
		Items []itemResponse `json:"items"`
	}

	areas := make([]areaResponse, len(summaries))
	for i, s := range summaries {
		items := make([]itemResponse, len(s.Items))
		for j, it := range s.Items {
			items[j] = itemResponse{Name: it.Name, Quantity: it.Quantity}
		}
		areas[i] = areaResponse{ID: s.Area.ID, Name: s.Area.Name, Items: items}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Areas []areaResponse `json:"areas"`
	}{Areas: areas})
}
