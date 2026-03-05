package vision

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// jsonExtractRe extracts a JSON object from a string that may contain surrounding prose or code fences.
var jsonExtractRe = regexp.MustCompile(`(?s)\{.*\}`)

// ParseResponse parses vision model response in format: name | quantity | notes
// One item per line.
func ParseResponse(raw string) []DetectedItem {
	lines := strings.Split(raw, "\n")
	items := make([]DetectedItem, 0)

	for _, line := range lines {
		if item := ParseLine(line); item != nil {
			items = append(items, *item)
		}
	}

	return items
}

// ParseLine parses a single "name | quantity | notes" line. Returns nil for
// blank lines and lines without a pipe separator (which are not item lines).
func ParseLine(line string) *DetectedItem {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// Lines without a pipe separator are not item lines (e.g. model preamble).
	// This structural check handles all such cases without needing an explicit
	// list of prefix phrases to skip.
	if !strings.Contains(line, "|") {
		return nil
	}

	parts := strings.Split(line, "|")
	item := DetectedItem{
		Name: strings.TrimSpace(parts[0]),
	}
	if len(parts) >= 2 {
		item.Quantity = strings.TrimSpace(parts[1])
	}
	if len(parts) >= 3 {
		item.Notes = strings.TrimSpace(parts[2])
	}

	if item.Name == "" {
		return nil
	}
	return &item
}

// jsonItem is the wire representation of a single item in the JSON response.
type jsonItem struct {
	Name     string `json:"name"`
	Quantity *int   `json:"quantity"`
	Notes    *string `json:"notes"`
}

// jsonResponse is the wire representation of the full JSON response.
type jsonResponse struct {
	Status *string    `json:"status"`
	Items  []jsonItem `json:"items"`
}

var validStatuses = map[AnalysisStatus]bool{
	StatusOK:      true,
	StatusNoItems: true,
	StatusNotFood: true,
	StatusUnclear: true,
}

// ParseJSONResponse parses a structured JSON response from the vision model.
// It extracts the JSON object liberally (tolerating surrounding prose or code
// fences), then validates the status enum and required fields before mapping
// to an AnalysisResult.
func ParseJSONResponse(raw string) (*AnalysisResult, error) {
	// Extract JSON object from potentially noisy response.
	jsonStr := jsonExtractRe.FindString(raw)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	var wire jsonResponse
	if err := json.Unmarshal([]byte(jsonStr), &wire); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if wire.Status == nil {
		return nil, fmt.Errorf("response missing required field: status")
	}
	if wire.Items == nil {
		return nil, fmt.Errorf("response missing required field: items")
	}

	status := AnalysisStatus(*wire.Status)
	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status value: %q", *wire.Status)
	}

	items := make([]DetectedItem, 0, len(wire.Items))
	for i, wi := range wire.Items {
		if wi.Name == "" {
			return nil, fmt.Errorf("item at index %d missing required field: name", i)
		}
		item := DetectedItem{Name: wi.Name}
		if wi.Quantity != nil {
			item.Quantity = strconv.Itoa(*wi.Quantity)
		}
		if wi.Notes != nil {
			item.Notes = *wi.Notes
		}
		items = append(items, item)
	}

	return &AnalysisResult{
		Status:      status,
		Items:       items,
		RawResponse: raw,
	}, nil
}
