package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/vbonduro/kitchinv/internal/service"
)

func (s *Server) handleListAreas(w http.ResponseWriter, r *http.Request) {
	areas, err := s.service.ListAreasWithItems(r.Context())
	if err != nil {
		http.Error(w, "failed to list areas", http.StatusInternalServerError)
		s.logger.Error("list areas failed", "error", err)
		return
	}

	if err := s.renderPage(w,
		map[string]any{"Areas": areas, "ActiveNav": "areas"},
		"base.html", "pages/areas.html", "partials/area_card.html",
	); err != nil {
		s.logger.Error("render page failed", "error", err)
	}
}

const maxAreaNameLen = 200

func (s *Server) handleCreateArea(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "area name required", http.StatusBadRequest)
		return
	}
	if len(name) > maxAreaNameLen {
		http.Error(w, "area name too long", http.StatusBadRequest)
		return
	}

	area, err := s.service.CreateArea(r.Context(), name)
	if err != nil {
		http.Error(w, "failed to create area", http.StatusInternalServerError)
		s.logger.Error("create area failed", "error", err)
		return
	}

	summary := &service.AreaSummary{Area: area}
	if err := s.renderPartial(w, "partials/area_card.html", summary); err != nil {
		s.logger.Error("render partial failed", "error", err)
	}
}

func (s *Server) handleGetAreaDetail(w http.ResponseWriter, r *http.Request) {
	areaID, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid area id", http.StatusBadRequest)
		return
	}

	area, items, photo, err := s.service.GetAreaWithItems(r.Context(), areaID)
	if err != nil {
		http.Error(w, "failed to get area", http.StatusInternalServerError)
		s.logger.Error("get area failed", "area_id", areaID, "error", err)
		return
	}
	if area == nil {
		http.NotFound(w, r)
		return
	}

	if err := s.renderPage(w,
		map[string]any{"Area": area, "Items": items, "Photo": photo, "ActiveNav": "areas"},
		"base.html", "pages/area_detail.html", "partials/item_list.html",
	); err != nil {
		s.logger.Error("render page failed", "error", err)
	}
}

func (s *Server) handleDeleteArea(w http.ResponseWriter, r *http.Request) {
	areaID, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid area id", http.StatusBadRequest)
		return
	}

	if err := s.service.DeleteArea(r.Context(), areaID); err != nil {
		http.Error(w, "failed to delete area", http.StatusInternalServerError)
		s.logger.Error("delete area failed", "area_id", areaID, "error", err)
		return
	}

	w.Header().Set("HX-Redirect", "/areas")
	w.WriteHeader(http.StatusOK)
}

// parseID extracts the {id} path variable and returns it as int64.
func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}
