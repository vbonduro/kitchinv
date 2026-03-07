package benchmark

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vbonduro/kitchinv/internal/vision"
)

func result(items ...vision.DetectedItem) *vision.AnalysisResult {
	return &vision.AnalysisResult{Status: vision.StatusOK, Items: items}
}

func item(name, qty string) vision.DetectedItem {
	return vision.DetectedItem{Name: name, Quantity: qty}
}

func gt(items ...GroundTruthItem) GroundTruth {
	return GroundTruth{Items: items}
}

func gti(name string, qty int) GroundTruthItem {
	return GroundTruthItem{Name: name, Quantity: qty}
}

func TestScorePerfectMatch(t *testing.T) {
	r := Score("fridge", gt(gti("Milk", 2), gti("Eggs", 12)),
		result(item("Milk", "2"), item("Eggs", "12")))

	assert.Equal(t, 2, r.Expected)
	assert.Equal(t, 2, r.Detected)
	assert.Equal(t, 2, r.ItemMatches)
	assert.Equal(t, 2, r.QuantityMatches)
	assert.InDelta(t, 1.0, r.ItemAccuracy, 0.001)
	assert.InDelta(t, 1.0, r.QuantityAccuracy, 0.001)
	assert.Empty(t, r.UnmatchedExpected)
	assert.Empty(t, r.ExtraDetected)
}

func TestScorePartialItemMatch(t *testing.T) {
	r := Score("fridge", gt(gti("Milk", 2), gti("Eggs", 12), gti("Butter", 1)),
		result(item("Milk", "2"), item("Eggs", "12")))

	assert.Equal(t, 3, r.Expected)
	assert.Equal(t, 2, r.ItemMatches)
	assert.InDelta(t, 2.0/3.0, r.ItemAccuracy, 0.001)
	assert.Equal(t, []string{"Butter"}, r.UnmatchedExpected)
	assert.Empty(t, r.ExtraDetected)
}

func TestScoreQuantityMismatch(t *testing.T) {
	r := Score("fridge", gt(gti("Milk", 2), gti("Eggs", 12)),
		result(item("Milk", "1"), item("Eggs", "12")))

	assert.Equal(t, 2, r.ItemMatches)
	assert.Equal(t, 1, r.QuantityMatches)
	assert.InDelta(t, 1.0, r.ItemAccuracy, 0.001)
	assert.InDelta(t, 0.5, r.QuantityAccuracy, 0.001)
}

func TestScoreCaseInsensitiveMatch(t *testing.T) {
	r := Score("fridge", gt(gti("milk", 1)), result(item("Milk", "1")))
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScoreSubstringMatch(t *testing.T) {
	// "Whole Milk" should match ground truth "Milk"
	r := Score("fridge", gt(gti("Milk", 1)), result(item("Whole Milk", "1")))
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScoreExtraDetected(t *testing.T) {
	r := Score("fridge", gt(gti("Milk", 1)),
		result(item("Milk", "1"), item("Mystery Sauce", "1")))

	assert.Equal(t, 1, r.ItemMatches)
	assert.Equal(t, []string{"Mystery Sauce"}, r.ExtraDetected)
}

func TestScoreNoItems(t *testing.T) {
	r := Score("fridge", gt(), result())
	assert.Equal(t, 0, r.Expected)
	assert.Equal(t, 0, r.Detected)
	assert.InDelta(t, 0.0, r.ItemAccuracy, 0.001)
	assert.InDelta(t, 0.0, r.QuantityAccuracy, 0.001)
}

func TestScoreNoMatches(t *testing.T) {
	r := Score("fridge", gt(gti("Milk", 1)), result(item("Cheese", "1")))
	assert.Equal(t, 0, r.ItemMatches)
	assert.InDelta(t, 0.0, r.ItemAccuracy, 0.001)
	assert.InDelta(t, 0.0, r.QuantityAccuracy, 0.001)
	assert.Equal(t, []string{"Milk"}, r.UnmatchedExpected)
	assert.Equal(t, []string{"Cheese"}, r.ExtraDetected)
}

func TestScoreEachGroundTruthMatchedOnce(t *testing.T) {
	// Two detected "Milk" items should only count as one match for one GT item.
	r := Score("fridge", gt(gti("Milk", 1)),
		result(item("Milk", "1"), item("Milk", "1")))

	assert.Equal(t, 1, r.ItemMatches)
	assert.Len(t, r.ExtraDetected, 1)
	assert.Equal(t, "Milk", r.ExtraDetected[0])
}
