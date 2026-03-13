package service

import (
	"strconv"
	"strings"

	"github.com/vbonduro/kitchinv/internal/vision"
)

type mergedItem struct {
	name     string
	quantity string
	bboxes   [][]float64
}

// mergeDetectedItems groups detected items by name (case-insensitive, trimmed),
// sums integer quantities, and collects all bboxes. Insertion order is preserved.
func mergeDetectedItems(detected []vision.DetectedItem) []mergedItem {
	index := make(map[string]int) // key → position in result
	var result []mergedItem

	for _, d := range detected {
		name := strings.TrimSpace(d.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)

		if pos, exists := index[key]; exists {
			// Merge into existing entry.
			existing := &result[pos]
			// Sum quantities if both are integers; otherwise keep first.
			if a, err1 := strconv.Atoi(existing.quantity); err1 == nil {
				if b, err2 := strconv.Atoi(d.Quantity); err2 == nil {
					existing.quantity = strconv.Itoa(a + b)
				}
			}
			if d.BBox != nil {
				existing.bboxes = append(existing.bboxes, []float64{d.BBox[0], d.BBox[1], d.BBox[2], d.BBox[3]})
			}
		} else {
			index[key] = len(result)
			item := mergedItem{
				name:     name,
				quantity: d.Quantity,
			}
			if d.BBox != nil {
				item.bboxes = [][]float64{{d.BBox[0], d.BBox[1], d.BBox[2], d.BBox[3]}}
			}
			result = append(result, item)
		}
	}

	return result
}
