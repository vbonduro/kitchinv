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
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip common headers or non-item lines
		if strings.HasPrefix(line, "Here") || strings.HasPrefix(line, "I see") || strings.HasPrefix(line, "Based on") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) >= 1 {
			item := DetectedItem{
				Name: strings.TrimSpace(parts[0]),
			}

			if len(parts) >= 2 {
				item.Quantity = strings.TrimSpace(parts[1])
			}
			if len(parts) >= 3 {
				item.Notes = strings.TrimSpace(parts[2])
			}

			if item.Name != "" {
				items = append(items, item)
			}
		}
	}

	return items
}
