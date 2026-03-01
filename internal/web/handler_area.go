package web

import (
	"encoding/json"
	"errors"
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

func (s *Server) handleUpdateArea(w http.ResponseWriter, r *http.Request) {
	areaID, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid area id", http.StatusBadRequest)
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		http.Error(w, "area name required", http.StatusBadRequest)
		return
	}
	if len(name) > maxAreaNameLen {
		http.Error(w, "area name too long", http.StatusBadRequest)
		return
	}

	area, err := s.service.UpdateArea(r.Context(), areaID, name)
	if err != nil {
		if errors.Is(err, service.ErrNameTaken) {
			http.Error(w, "an area with this name already exists", http.StatusConflict)
			return
		}
		http.Error(w, "failed to update area", http.StatusInternalServerError)
		s.logger.Error("update area failed", "area_id", areaID, "error", err)
		return
	}

	_, areaItems, areaPhoto, err := s.service.GetAreaWithItems(r.Context(), areaID)
	if err != nil {
		http.Error(w, "failed to get area details", http.StatusInternalServerError)
		s.logger.Error("get area failed after update", "area_id", areaID, "error", err)
		return
	}

	summary := &service.AreaSummary{Area: area, Photo: areaPhoto, Items: areaItems}
	if err := s.renderPartial(w, "partials/area_card.html", summary); err != nil {
		s.logger.Error("render partial failed", "error", err)
	}
}

func (s *Server) handleDeletePhoto(w http.ResponseWriter, r *http.Request) {
	areaID, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid area id", http.StatusBadRequest)
		return
	}

	if err := s.service.DeletePhoto(r.Context(), areaID); err != nil {
		http.Error(w, "failed to delete photo", http.StatusInternalServerError)
		s.logger.Error("delete photo failed", "area_id", areaID, "error", err)
		return
	}

	// Return updated card (no photo, no items).
	area, err := s.service.GetArea(r.Context(), areaID)
	if err != nil || area == nil {
		http.Error(w, "failed to get area", http.StatusInternalServerError)
		return
	}

	summary := &service.AreaSummary{Area: area}
	if err := s.renderPartial(w, "partials/area_card.html", summary); err != nil {
		s.logger.Error("render partial failed", "error", err)
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

	// Return empty body — HTMX will remove the card from the DOM.
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGetAreaItems(w http.ResponseWriter, r *http.Request) {
	areaID, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid area id", http.StatusBadRequest)
		return
	}

	_, items, _, err := s.service.GetAreaWithItems(r.Context(), areaID)
	if err != nil {
		http.Error(w, "failed to get items", http.StatusInternalServerError)
		s.logger.Error("get area items failed", "area_id", areaID, "error", err)
		return
	}

	data := map[string]any{"AreaID": areaID, "Items": items}
	if err := s.renderPartial(w, "partials/item_list.html", data); err != nil {
		s.logger.Error("render partial failed", "error", err)
	}
}

func (s *Server) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	areaID, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid area id", http.StatusBadRequest)
		return
	}

	var body struct {
		Name     string `json:"name"`
		Quantity string `json:"quantity"`
		Notes    string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		http.Error(w, "item name required", http.StatusBadRequest)
		return
	}

	item, err := s.service.CreateItem(r.Context(), areaID, name, strings.TrimSpace(body.Quantity), strings.TrimSpace(body.Notes))
	if err != nil {
		http.Error(w, "failed to create item", http.StatusInternalServerError)
		s.logger.Error("create item failed", "area_id", areaID, "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}

func (s *Server) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	_, err := parseID(r) // areaID — validates the path
	if err != nil {
		http.Error(w, "invalid area id", http.StatusBadRequest)
		return
	}

	itemID, err := parseItemID(r)
	if err != nil {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	var body struct {
		Name     string `json:"name"`
		Quantity string `json:"quantity"`
		Notes    string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		http.Error(w, "item name required", http.StatusBadRequest)
		return
	}

	item, err := s.service.UpdateItem(r.Context(), itemID, name, strings.TrimSpace(body.Quantity), strings.TrimSpace(body.Notes))
	if err != nil {
		http.Error(w, "failed to update item", http.StatusInternalServerError)
		s.logger.Error("update item failed", "item_id", itemID, "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}

func (s *Server) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	_, err := parseID(r) // areaID — validates the path
	if err != nil {
		http.Error(w, "invalid area id", http.StatusBadRequest)
		return
	}

	itemID, err := parseItemID(r)
	if err != nil {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	if err := s.service.DeleteItem(r.Context(), itemID); err != nil {
		http.Error(w, "failed to delete item", http.StatusInternalServerError)
		s.logger.Error("delete item failed", "item_id", itemID, "error", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// parseID extracts the {id} path variable and returns it as int64.
func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// parseItemID extracts the {itemId} path variable and returns it as int64.
func parseItemID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("itemId"), 10, 64)
}
