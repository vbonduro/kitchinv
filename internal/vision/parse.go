package vision

import (
	"strings"
)

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
// blank lines, header lines, and lines with no name.
func ParseLine(line string) *DetectedItem {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// Skip common headers or non-item lines
	if strings.HasPrefix(line, "Here") || strings.HasPrefix(line, "I see") || strings.HasPrefix(line, "Based on") {
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
