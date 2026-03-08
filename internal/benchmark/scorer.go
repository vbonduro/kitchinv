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

// tokenOverlapThreshold is the minimum Jaccard similarity required for a token
// overlap match. 0.2 means at least 1 shared meaningful word out of 5 distinct.
const tokenOverlapThreshold = 0.2

// stopWords are common words excluded from token matching to avoid false positives.
var stopWords = map[string]bool{
	// Articles / prepositions (including French ones that appear in brand names)
	"a": true, "an": true, "the": true, "of": true, "in": true,
	"and": true, "with": true, "for": true, "sans": true,
	"de": true, "le": true, "la": true,
	// Generic container / packaging words
	"box": true, "bag": true, "pack": true, "package": true,
	"can": true, "jar": true, "bottle": true, "carton": true, "tub": true,
	// Generic preparation / state descriptors
	"mix": true, "frozen": true, "canned": true, "free": true,
	"fresh": true, "dried": true, "plain": true, "original": true,
	"organic": true, "whole": true, "light": true, "dark": true,
	"small": true, "large": true, "medium": true,
	// Generic food category words that appear in many unrelated items
	"sauce": true, "drink": true, "beverage": true,
	// Ice is too generic — appears in "ice cream", "ice pops", "iced tea" etc.
	"ice": true, "cream": true,
	// Colours used as descriptors in brand names
	"red": true, "white": true, "green": true, "blue": true,
	// Hot/cold descriptors
	"hot": true, "cold": true,
	// Long brand name filler words
	"cafe": true, "torrefaction": true, "francaise": true, "francai": true,
	"fine": true, "filtered": true, "lactose": true,
	// Generic food preparation / container descriptors (stemmed forms)
	"packag": true, // "packaged" stems to "packag"
	// Generic food category words too broad to be identifying on their own
	"cheese": true,
	// Generic soup/stew words that inflate union without helping identification
	"soup": true, "noodle": true, "broth": true,
	// Brand-name structural words
	"own": true, "earth": true,
	// Chip is too ambiguous (chocolate chip vs potato chip)
	"chip": true,
	// Short brand abbreviations / articles that cause false matches
	"pc": true, "st": true,
}

// namesMatch returns true if either name contains the other (case-insensitive),
// or if the token Jaccard similarity meets tokenOverlapThreshold.
func namesMatch(a, b string) bool {
	al := strings.ToLower(strings.TrimSpace(a))
	bl := strings.ToLower(strings.TrimSpace(b))
	if strings.Contains(al, bl) || strings.Contains(bl, al) {
		return true
	}
	return tokenJaccard(al, bl) >= tokenOverlapThreshold
}

// tokenJaccard computes Jaccard similarity on the word-token sets of two strings,
// excluding stop words.
func tokenJaccard(a, b string) float64 {
	ta := tokenSet(a)
	tb := tokenSet(b)
	if len(ta) == 0 && len(tb) == 0 {
		return 0
	}
	intersection := 0
	for w := range ta {
		if tb[w] {
			intersection++
		}
	}
	union := len(ta) + len(tb) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// accentMap maps accented runes to their ASCII equivalents for normalization.
var accentMap = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'ø': 'o',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
	'ñ': 'n', 'ç': 'c', 'ý': 'y', 'ß': 's',
}

// normalizeRunes replaces accented characters with their ASCII equivalents.
func normalizeRunes(s string) string {
	var b strings.Builder
	for _, r := range s {
		if a, ok := accentMap[r]; ok {
			b.WriteRune(a)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// stem applies simple suffix normalization to reduce inflected forms:
// -ies plurals, sibilant -es plurals, regular -s plurals, possessives,
// and common spelling/compound variants.
func stem(w string) string {
	// Possessive: strip trailing 's (frank's→frank)
	w = strings.TrimSuffix(w, "'s")
	switch {
	case w == "lasagne":
		return "lasagna"
	case w == "popsicle" || w == "popsicles":
		return "pop"
	case w == "redhot":
		return "hot" // will be a stop word — effectively strips it; frank still matches
	case len(w) > 4 && strings.HasSuffix(w, "ies"):
		return w[:len(w)-3] + "y" // berries→berry, babies→baby
	case len(w) > 4 && (strings.HasSuffix(w, "shes") ||
		strings.HasSuffix(w, "ches") ||
		strings.HasSuffix(w, "xes") ||
		strings.HasSuffix(w, "zes")):
		return w[:len(w)-2] // dishes→dish, patches→patch
	case len(w) > 5 && strings.HasSuffix(w, "ed"):
		base := w[:len(w)-2]
		// collapse doubled final consonant: shredded→shreDD→shred
		if len(base) >= 2 && base[len(base)-1] == base[len(base)-2] {
			base = base[:len(base)-1]
		}
		return base
	case len(w) > 2 && strings.HasSuffix(w, "s"):
		return w[:len(w)-1] // pickles→pickle, extracts→extract, noodles→noodle
	}
	return w
}

// tokenSet splits s into lowercase, accent-normalized, stemmed words and returns
// the set, excluding stop words and single-character tokens.
// Slash-separated compounds (e.g. "Spices/Extracts") are split into sub-tokens.
func tokenSet(s string) map[string]bool {
	s = normalizeRunes(strings.ToLower(strings.TrimSpace(s)))
	set := make(map[string]bool)
	for _, w := range strings.Fields(s) {
		w = strings.TrimRight(w, ".,;:!?'\"/)")
		w = strings.TrimLeft(w, "('\"")
		// split slash-compounds into individual tokens
		parts := strings.Split(w, "/")
		for _, p := range parts {
			p = stem(p)
			if len(p) <= 1 || stopWords[p] {
				continue
			}
			set[p] = true
		}
	}
	return set
}

// quantityEqual returns true if the detected quantity string equals the expected int.
func quantityEqual(expected int, detected string) bool {
	d, err := strconv.Atoi(strings.TrimSpace(detected))
	if err != nil {
		return false
	}
	return d == expected
}
