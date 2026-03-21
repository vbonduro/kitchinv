package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/vbonduro/kitchinv/internal/domain"
)

func (s *Server) handleListOverrides(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rules, err := s.service.ListOverrideRules(ctx)
	if err != nil {
		http.Error(w, "failed to list override rules", http.StatusInternalServerError)
		s.logger.Error("list override rules failed", "error", err)
		return
	}

	areas, err := s.service.ListAreas(ctx)
	if err != nil {
		http.Error(w, "failed to list areas", http.StatusInternalServerError)
		s.logger.Error("list areas failed", "error", err)
		return
	}

	areaMap := make(map[int64]string, len(areas))
	for _, a := range areas {
		areaMap[a.ID] = a.Name
	}

	if err := s.renderPage(w, map[string]any{
		"Rules":     rules,
		"Areas":     areas,
		"AreaMap":   areaMap,
		"ActiveNav": "overrides",
	}, "base.html", "pages/overrides.html"); err != nil {
		s.logger.Error("render page failed", "error", err)
	}
}

func (s *Server) handleCreateOverride(w http.ResponseWriter, r *http.Request) {
	rule, ok := s.parseOverrideForm(w, r)
	if !ok {
		return
	}

	if _, err := s.service.CreateOverrideRule(r.Context(), rule); err != nil {
		http.Error(w, "failed to create override rule", http.StatusInternalServerError)
		s.logger.Error("create override rule failed", "error", err)
		return
	}

	http.Redirect(w, r, "/overrides", http.StatusSeeOther)
}

func (s *Server) handleUpdateOverride(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid override id", http.StatusBadRequest)
		return
	}

	rule, ok := s.parseOverrideForm(w, r)
	if !ok {
		return
	}
	rule.ID = id

	if _, err := s.service.UpdateOverrideRule(r.Context(), rule); err != nil {
		http.Error(w, "failed to update override rule", http.StatusInternalServerError)
		s.logger.Error("update override rule failed", "id", id, "error", err)
		return
	}

	http.Redirect(w, r, "/overrides", http.StatusSeeOther)
}

func (s *Server) handleDeleteOverride(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid override id", http.StatusBadRequest)
		return
	}

	if err := s.service.DeleteOverrideRule(r.Context(), id); err != nil {
		http.Error(w, "failed to delete override rule", http.StatusInternalServerError)
		s.logger.Error("delete override rule failed", "id", id, "error", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReorderOverrides(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := s.service.ReorderOverrideRules(r.Context(), body.IDs); err != nil {
		http.Error(w, "failed to reorder", http.StatusInternalServerError)
		s.logger.Error("reorder override rules failed", "error", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// parseOverrideForm parses and validates the common override rule form fields.
// Reports validation errors directly to w and returns false on failure.
func (s *Server) parseOverrideForm(w http.ResponseWriter, r *http.Request) (domain.OverrideRule, bool) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return domain.OverrideRule{}, false
	}

	pattern := strings.TrimSpace(r.FormValue("match_pattern"))
	if pattern == "" {
		http.Error(w, "match_pattern is required", http.StatusBadRequest)
		return domain.OverrideRule{}, false
	}

	matchExact := r.FormValue("match_exact") == "on"
	matchCI := r.FormValue("match_case_insensitive") == "on"
	matchSub := r.FormValue("match_substring") == "on"

	if !matchExact && !matchSub {
		http.Error(w, "at least one match mode must be selected", http.StatusBadRequest)
		return domain.OverrideRule{}, false
	}

	scope := r.FormValue("scope")
	if scope != "global" && scope != "area" {
		scope = "global"
	}

	var areaIDs []int64
	if scope == "area" {
		for _, v := range r.Form["area_ids[]"] {
			id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err == nil {
				areaIDs = append(areaIDs, id)
			}
		}
		if len(areaIDs) == 0 {
			http.Error(w, "area_ids required when scope is area", http.StatusBadRequest)
			return domain.OverrideRule{}, false
		}
	}

	sortOrder := 0
	if so := strings.TrimSpace(r.FormValue("sort_order")); so != "" {
		if n, err := strconv.Atoi(so); err == nil {
			sortOrder = n
		}
	}

	return domain.OverrideRule{
		MatchPattern:         pattern,
		Replacement:          r.FormValue("replacement"),
		MatchExact:           matchExact,
		MatchCaseInsensitive: matchCI,
		MatchSubstring:       matchSub,
		Scope:                scope,
		AreaIDs:              areaIDs,
		SortOrder:            sortOrder,
	}, true
}
