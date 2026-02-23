package web

import (
	"net/http"
	"strings"

	"github.com/vbonduro/kitchinv/internal/domain"
)

const maxSearchQueryLen = 200

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > maxSearchQueryLen {
		query = query[:maxSearchQueryLen]
	}

	var items []*domain.Item
	if query != "" {
		var err error
		items, err = s.service.SearchItems(r.Context(), query)
		if err != nil {
			http.Error(w, "search failed", http.StatusInternalServerError)
			s.logger.Error("search failed", "query", query, "error", err)
			return
		}
	}

	// HTMX partial update: return only results fragment.
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Cache-Control", "no-store")
		if err := s.renderPartial(w, "partials/search_results.html", items); err != nil {
			s.logger.Error("render partial failed", "error", err)
		}
		return
	}

	if err := s.renderPage(w,
		map[string]any{"Results": items, "Query": query, "ActiveNav": "search"},
		"base.html", "pages/search.html", "partials/search_results.html",
	); err != nil {
		s.logger.Error("render page failed", "error", err)
	}
}
