package benchmark

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		result(item("Milk", "2"), item("Eggs", "12")), nil)

	assert.Equal(t, 2, r.Expected)
	assert.Equal(t, 2, r.Detected)
	assert.Equal(t, 2, r.ItemMatches)
	assert.Equal(t, 2, r.QuantityMatches)
	assert.InDelta(t, 1.0, r.ItemAccuracy, 0.001)
	assert.InDelta(t, 1.0, r.QuantityAccuracy, 0.001)
	assert.Empty(t, r.Extra)

	require.Len(t, r.Items, 2)
	assert.NotNil(t, r.Items[0].Detected)
	assert.True(t, r.Items[0].QuantityMatch)
	assert.NotNil(t, r.Items[1].Detected)
	assert.True(t, r.Items[1].QuantityMatch)
}

func TestScorePartialItemMatch(t *testing.T) {
	r := Score("fridge", gt(gti("Milk", 2), gti("Eggs", 12), gti("Butter", 1)),
		result(item("Milk", "2"), item("Eggs", "12")), nil)

	assert.Equal(t, 3, r.Expected)
	assert.Equal(t, 2, r.ItemMatches)
	assert.InDelta(t, 2.0/3.0, r.ItemAccuracy, 0.001)
	assert.Empty(t, r.Extra)

	require.Len(t, r.Items, 3)
	assert.NotNil(t, r.Items[0].Detected)
	assert.NotNil(t, r.Items[1].Detected)
	assert.Nil(t, r.Items[2].Detected)
	assert.Equal(t, "Butter", r.Items[2].Expected.Name)
}

func TestScoreQuantityMismatch(t *testing.T) {
	r := Score("fridge", gt(gti("Milk", 2), gti("Eggs", 12)),
		result(item("Milk", "1"), item("Eggs", "12")), nil)

	assert.Equal(t, 2, r.ItemMatches)
	assert.Equal(t, 1, r.QuantityMatches)
	assert.InDelta(t, 1.0, r.ItemAccuracy, 0.001)
	assert.InDelta(t, 0.5, r.QuantityAccuracy, 0.001)

	require.Len(t, r.Items, 2)
	assert.NotNil(t, r.Items[0].Detected)
	assert.False(t, r.Items[0].QuantityMatch)
	assert.Equal(t, "1", r.Items[0].Detected.Quantity)
	assert.NotNil(t, r.Items[1].Detected)
	assert.True(t, r.Items[1].QuantityMatch)
}

func TestScoreCaseInsensitiveMatch(t *testing.T) {
	r := Score("fridge", gt(gti("milk", 1)), result(item("Milk", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
	require.Len(t, r.Items, 1)
	assert.NotNil(t, r.Items[0].Detected)
	assert.Equal(t, "Milk", r.Items[0].Detected.Name)
}

func TestScoreSubstringMatch(t *testing.T) {
	r := Score("fridge", gt(gti("Milk", 1)), result(item("Whole Milk", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
	require.Len(t, r.Items, 1)
	assert.NotNil(t, r.Items[0].Detected)
	assert.Equal(t, "Whole Milk", r.Items[0].Detected.Name)
}

func TestScoreExtraDetected(t *testing.T) {
	r := Score("fridge", gt(gti("Milk", 1)),
		result(item("Milk", "1"), item("Mystery Sauce", "1")), nil)

	assert.Equal(t, 1, r.ItemMatches)
	require.Len(t, r.Extra, 1)
	assert.Equal(t, "Mystery Sauce", r.Extra[0].Name)
	assert.Equal(t, "1", r.Extra[0].Quantity)
}

func TestScoreNoItems(t *testing.T) {
	r := Score("fridge", gt(), result(), nil)
	assert.Equal(t, 0, r.Expected)
	assert.Equal(t, 0, r.Detected)
	assert.InDelta(t, 0.0, r.ItemAccuracy, 0.001)
	assert.InDelta(t, 0.0, r.QuantityAccuracy, 0.001)
	assert.Empty(t, r.Items)
	assert.Empty(t, r.Extra)
}

func TestScoreNoMatches(t *testing.T) {
	r := Score("fridge", gt(gti("Milk", 1)), result(item("Cheese", "1")), nil)
	assert.Equal(t, 0, r.ItemMatches)
	assert.InDelta(t, 0.0, r.ItemAccuracy, 0.001)
	assert.InDelta(t, 0.0, r.QuantityAccuracy, 0.001)

	require.Len(t, r.Items, 1)
	assert.Nil(t, r.Items[0].Detected)
	assert.Equal(t, "Milk", r.Items[0].Expected.Name)

	require.Len(t, r.Extra, 1)
	assert.Equal(t, "Cheese", r.Extra[0].Name)
}

func TestScoreEachGroundTruthMatchedOnce(t *testing.T) {
	r := Score("fridge", gt(gti("Milk", 1)),
		result(item("Milk", "1"), item("Milk", "1")), nil)

	assert.Equal(t, 1, r.ItemMatches)
	require.Len(t, r.Extra, 1)
	assert.Equal(t, "Milk", r.Extra[0].Name)
}

func TestScoreItemResultCarriesDetectedName(t *testing.T) {
	r := Score("fridge", gt(gti("Milk", 2)),
		result(item("Organic Whole Milk", "2")), nil)

	require.Len(t, r.Items, 1)
	assert.Equal(t, "Milk", r.Items[0].Expected.Name)
	assert.Equal(t, 2, r.Items[0].Expected.Quantity)
	require.NotNil(t, r.Items[0].Detected)
	assert.Equal(t, "Organic Whole Milk", r.Items[0].Detected.Name)
	assert.Equal(t, "2", r.Items[0].Detected.Quantity)
	assert.True(t, r.Items[0].QuantityMatch)
}

func TestScoreOverrideMatch(t *testing.T) {
	overrides := LoadOverrides([]Override{
		{Fixture: "fridge", Expected: "popsicles", Detected: "Freeze Pops"},
	})
	r := Score("fridge", gt(gti("popsicles", 20)),
		result(item("Freeze Pops", "20")), overrides)

	require.Len(t, r.Items, 1)
	assert.NotNil(t, r.Items[0].Detected)
	assert.True(t, r.Items[0].OverrideApplied)
	assert.True(t, r.Items[0].QuantityMatch)
	assert.Equal(t, "Freeze Pops", r.Items[0].Detected.Name)
}

func TestScoreOverrideDoesNotApplyToOtherFixture(t *testing.T) {
	overrides := LoadOverrides([]Override{
		{Fixture: "freezer", Expected: "popsicles", Detected: "Freeze Pops"},
	})
	// Override is for "freezer" fixture, not "fridge" — should not match.
	r := Score("fridge", gt(gti("popsicles", 20)),
		result(item("Freeze Pops", "20")), overrides)

	require.Len(t, r.Items, 1)
	assert.Nil(t, r.Items[0].Detected)
}

func TestScoreSubstringTakesPriorityOverOverride(t *testing.T) {
	overrides := LoadOverrides([]Override{
		{Fixture: "fridge", Expected: "Milk", Detected: "Oat Milk"},
	})
	// "Whole Milk" substring-matches "Milk" in pass 1, override never fires.
	r := Score("fridge", gt(gti("Milk", 1)),
		result(item("Whole Milk", "1"), item("Oat Milk", "1")), overrides)

	require.Len(t, r.Items, 1)
	assert.NotNil(t, r.Items[0].Detected)
	assert.False(t, r.Items[0].OverrideApplied)
	assert.Equal(t, "Whole Milk", r.Items[0].Detected.Name)
}
