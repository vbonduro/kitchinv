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
	// UnmatchedExpected lists ground truth items not found in the model output.
	UnmatchedExpected []string `json:"unmatched_expected"`
	// ExtraDetected lists model-detected items not in the ground truth.
	ExtraDetected []string `json:"extra_detected"`
}

// Score compares a vision AnalysisResult against a GroundTruth and returns a
// MatchResult. Name matching is case-insensitive substring: a detected item
// matches a ground truth item if either name contains the other. Each ground
// truth item is matched at most once (first match wins).
func Score(fixture string, gt GroundTruth, result *vision.AnalysisResult) MatchResult {
	matched := make([]bool, len(gt.Items))
	detectedUsed := make([]bool, len(result.Items))
	quantityMatches := 0

	for i, expected := range gt.Items {
		for j, detected := range result.Items {
			if detectedUsed[j] {
				continue
			}
			if namesMatch(expected.Name, detected.Name) {
				matched[i] = true
				detectedUsed[j] = true
				if quantityEqual(expected.Quantity, detected.Quantity) {
					quantityMatches++
				}
				break
			}
		}
	}

	itemMatches := 0
	unmatched := []string{}
	for i, m := range matched {
		if m {
			itemMatches++
		} else {
			unmatched = append(unmatched, gt.Items[i].Name)
		}
	}

	extra := []string{}
	for j, used := range detectedUsed {
		if !used {
			extra = append(extra, result.Items[j].Name)
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
		Fixture:           fixture,
		Expected:          len(gt.Items),
		Detected:          len(result.Items),
		ItemMatches:       itemMatches,
		QuantityMatches:   quantityMatches,
		ItemAccuracy:      itemAccuracy,
		QuantityAccuracy:  quantityAccuracy,
		UnmatchedExpected: unmatched,
		ExtraDetected:     extra,
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
