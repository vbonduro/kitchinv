// Package benchmark provides scoring logic for comparing vision analysis
// results against a known ground truth.
package benchmark

import (
	"strconv"
	"strings"

	"github.com/vbonduro/kitchinv/internal/vision"
)

// GroundTruthItem is a single expected item in a benchmark fixture.
type GroundTruthItem struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

// GroundTruth is the expected output for a single fixture image.
type GroundTruth struct {
	Items []GroundTruthItem `json:"items"`
}

// Override declares that a ground truth name should be considered a match for
// a specific detected name within a given fixture, even if substring matching
// would not connect them.
type Override struct {
	Fixture  string `json:"fixture"`
	Expected string `json:"expected"`
	Detected string `json:"detected"`
}

// Overrides is a collection of Override rules indexed for fast lookup.
// Key: "fixture\x00expected_lower" → detected_lower
type Overrides map[string]string

// LoadOverrides builds an Overrides lookup from a slice of Override rules.
func LoadOverrides(rules []Override) Overrides {
	o := make(Overrides, len(rules))
	for _, r := range rules {
		key := overrideKey(r.Fixture, r.Expected)
		o[key] = strings.ToLower(strings.TrimSpace(r.Detected))
	}
	return o
}

func overrideKey(fixture, expected string) string {
	return fixture + "\x00" + strings.ToLower(strings.TrimSpace(expected))
}

// ItemResult records the comparison outcome for one ground truth item.
type ItemResult struct {
	// Expected is the ground truth item.
	Expected GroundTruthItem `json:"expected"`
	// Detected is the model's matched item, nil if unmatched.
	Detected *vision.DetectedItem `json:"detected,omitempty"`
	// QuantityMatch is true when names matched and quantities agreed.
	QuantityMatch bool `json:"quantity_match"`
	// OverrideApplied is true when the match was made via an override rule.
	OverrideApplied bool `json:"override_applied,omitempty"`
}

// ExtraItem is a model-detected item that had no ground truth counterpart.
type ExtraItem struct {
	Name     string `json:"name"`
	Quantity string `json:"quantity"`
}

// MatchResult holds the scoring outcome for a single fixture.
type MatchResult struct {
	// Fixture is the name of the fixture directory.
	Fixture string `json:"fixture"`
	// Expected is the number of ground truth items.
	Expected int `json:"expected"`
	// Detected is the number of items the model returned.
	Detected int `json:"detected"`
	// ItemMatches is the number of ground truth items matched by name.
	ItemMatches int `json:"item_matches"`
	// QuantityMatches is the number of matched items where quantity was also correct.
	QuantityMatches int `json:"quantity_matches"`
	// ItemAccuracy is ItemMatches / Expected (0–1).
	ItemAccuracy float64 `json:"item_accuracy"`
	// QuantityAccuracy is QuantityMatches / ItemMatches (0–1), or 0 if no matches.
	QuantityAccuracy float64 `json:"quantity_accuracy"`
	// Items is the per-item comparison: one entry per ground truth item.
	Items []ItemResult `json:"items"`
	// Extra lists model-detected items that had no ground truth match.
	Extra []ExtraItem `json:"extra"`
}

// Score compares a vision AnalysisResult against a GroundTruth and returns a
// MatchResult. Matching is done in two passes:
//  1. Substring match: case-insensitive, either name contains the other.
//  2. Override match: explicit rules from overrides (may be nil).
//
// Each ground truth item is matched at most once (first match wins).
func Score(fixture string, gt GroundTruth, result *vision.AnalysisResult, overrides Overrides) MatchResult {
	detectedUsed := make([]bool, len(result.Items))
	itemResults := make([]ItemResult, len(gt.Items))
	quantityMatches := 0

	for i, expected := range gt.Items {
		ir := ItemResult{Expected: expected}

		// Pass 1: substring match.
		for j, detected := range result.Items {
			if detectedUsed[j] {
				continue
			}
			if namesMatch(expected.Name, detected.Name) {
				d := detected
				ir.Detected = &d
				detectedUsed[j] = true
				if quantityEqual(expected.Quantity, detected.Quantity) {
					ir.QuantityMatch = true
					quantityMatches++
				}
				break
			}
		}

		// Pass 2: override match (only if not already matched).
		if ir.Detected == nil && overrides != nil {
			if detectedName, ok := overrides[overrideKey(fixture, expected.Name)]; ok {
				for j, detected := range result.Items {
					if detectedUsed[j] {
						continue
					}
					if strings.ToLower(strings.TrimSpace(detected.Name)) == detectedName {
						d := detected
						ir.Detected = &d
						ir.OverrideApplied = true
						detectedUsed[j] = true
						if quantityEqual(expected.Quantity, detected.Quantity) {
							ir.QuantityMatch = true
							quantityMatches++
						}
						break
					}
				}
			}
		}

		itemResults[i] = ir
	}

	itemMatches := 0
	for _, ir := range itemResults {
		if ir.Detected != nil {
			itemMatches++
		}
	}

	extra := []ExtraItem{}
	for j, used := range detectedUsed {
		if !used {
			extra = append(extra, ExtraItem{
				Name:     result.Items[j].Name,
				Quantity: result.Items[j].Quantity,
			})
		}
	}

	itemAccuracy := 0.0
	if len(gt.Items) > 0 {
		itemAccuracy = float64(itemMatches) / float64(len(gt.Items))
	}

	quantityAccuracy := 0.0
	if itemMatches > 0 {
		quantityAccuracy = float64(quantityMatches) / float64(itemMatches)
	}

	return MatchResult{
		Fixture:          fixture,
		Expected:         len(gt.Items),
		Detected:         len(result.Items),
		ItemMatches:      itemMatches,
		QuantityMatches:  quantityMatches,
		ItemAccuracy:     itemAccuracy,
		QuantityAccuracy: quantityAccuracy,
		Items:            itemResults,
		Extra:            extra,
	}
}

// namesMatch returns true if either name contains the other, case-insensitively.
func namesMatch(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	return strings.Contains(a, b) || strings.Contains(b, a)
}

// quantityEqual returns true if the detected quantity string equals the expected int.
func quantityEqual(expected int, detected string) bool {
	d, err := strconv.Atoi(strings.TrimSpace(detected))
	if err != nil {
		return false
	}
	return d == expected
}
