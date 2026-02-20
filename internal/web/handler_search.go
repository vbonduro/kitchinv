package web

import (
	"log"
	"net/http"
	"strings"

	"github.com/vbonduro/kitchinv/internal/domain"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	var items []*domain.Item
	if query != "" {
		var err error
		items, err = s.service.SearchItems(r.Context(), query)
		if err != nil {
			http.Error(w, "search failed", http.StatusInternalServerError)
			log.Printf("search error: %v", err)
			return
		}
	}

	// HTMX partial update: return only results fragment.
	if r.Header.Get("HX-Request") == "true" {
		if err := s.renderPartial(w, "partials/search_results.html", items); err != nil {
			log.Printf("render partial error: %v", err)
		}
		return
	}

	if err := s.renderPage(w,
		map[string]any{"Results": items, "Query": query, "ActiveNav": "search"},
		"base.html", "pages/search.html", "partials/search_results.html",
	); err != nil {
		log.Printf("render page error: %v", err)
	}
}
