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
	// Use a pair with no natural token/substring overlap so override is required.
	overrides := LoadOverrides([]Override{
		{Fixture: "fridge", Expected: "chipotle aioli", Detected: "Garlic Dip"},
	})
	r := Score("fridge", gt(gti("chipotle aioli", 1)),
		result(item("Garlic Dip", "1")), overrides)

	require.Len(t, r.Items, 1)
	assert.NotNil(t, r.Items[0].Detected)
	assert.True(t, r.Items[0].OverrideApplied)
	assert.True(t, r.Items[0].QuantityMatch)
	assert.Equal(t, "Garlic Dip", r.Items[0].Detected.Name)
}

func TestScoreOverrideDoesNotApplyToOtherFixture(t *testing.T) {
	overrides := LoadOverrides([]Override{
		{Fixture: "freezer", Expected: "chipotle aioli", Detected: "Garlic Dip"},
	})
	// Override is for "freezer" fixture, not "fridge" — should not match.
	r := Score("fridge", gt(gti("chipotle aioli", 1)),
		result(item("Garlic Dip", "1")), overrides)

	require.Len(t, r.Items, 1)
	assert.Nil(t, r.Items[0].Detected)
}

func TestScoreTokenOverlapMatch(t *testing.T) {
	// "Social Vodka Soda" shares "social" and "vodka" with "social vodka drink" — 2/4 overlap
	r := Score("fridge", gt(gti("social vodka drink", 1)),
		result(item("Social Vodka Soda", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
	require.Len(t, r.Items, 1)
	assert.NotNil(t, r.Items[0].Detected)
}

func TestScoreTokenOverlapNoMatchBelowThreshold(t *testing.T) {
	r := Score("fridge", gt(gti("chicken broth", 1)),
		result(item("Tomato Sauce", "1")), nil)
	assert.Equal(t, 0, r.ItemMatches)
	require.Len(t, r.Items, 1)
	assert.Nil(t, r.Items[0].Detected)
}

func TestScoreNoFalsePositiveSauceToken(t *testing.T) {
	// "sauce" is a stop word — "soy sauce" must not match "Frank's RedHot Sauce"
	r := Score("fridge", gt(gti("soy sauce", 1)),
		result(item("Frank's RedHot Sauce", "1")), nil)
	assert.Equal(t, 0, r.ItemMatches)
}


func TestScoreNoFalsePositiveGreenToken(t *testing.T) {
	// "green" is a stop word — "food colouring green" must not match "Canned Green Peas"
	r := Score("pantry", gt(gti("food colouring green", 1)),
		result(item("Canned Green Peas", "1")), nil)
	assert.Equal(t, 0, r.ItemMatches)
}

func TestScoreTokenOverlapYakisoba(t *testing.T) {
	// "Yakisoba Frozen Meal" shares "yakisoba" with "yakisoba noodles box" — 1/3 Jaccard
	r := Score("freezer", gt(gti("yakisoba noodles box", 1)),
		result(item("Yakisoba Frozen Meal", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScoreTokenOverlapHeineken(t *testing.T) {
	// "Heineken sans alcool" shares "heineken" with "alcohol free heineken" — 1/3 Jaccard
	r := Score("fridge", gt(gti("alcohol free heineken", 1)),
		result(item("Heineken sans alcool", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScoreUnicodeNormalization(t *testing.T) {
	// "iögo nanö" should match "iogo nano" after unicode normalization
	r := Score("fridge", gt(gti("iogo nano yogurt drink pack", 1)),
		result(item("Iögo Nanö Yogurt Tubes", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScorePluralStemming(t *testing.T) {
	// "pickles" should match "pickle jar" after plural stripping
	r := Score("fridge", gt(gti("pickle jar", 1)),
		result(item("Pickles", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScoreSpellingVariantLasagne(t *testing.T) {
	// "lasagne" (Italian) should match "lasagna noodles" after normalization
	r := Score("pantry", gt(gti("lasagna noodles", 1)),
		result(item("Kirkland Italian Pasta Lasagne", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScorePluralExtract(t *testing.T) {
	// "extracts" should match "almond extract"
	r := Score("pantry", gt(gti("almond extract", 1)),
		result(item("Variety of Spices/Extracts", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScoreFranksRedHot(t *testing.T) {
	// "Frank's" apostrophe-s must not block token match with "franks"
	r := Score("fridge", gt(gti("franks red hot sauce", 1)),
		result(item("Frank's RedHot Sauce", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScoreNoFalsePositiveIceCream(t *testing.T) {
	// "vanilla ice cream" must not match "Ice Pops" — ice/cream are stop words
	r := Score("freezer", gt(gti("vanilla ice cream", 1), gti("popsicles", 1)),
		result(item("Ice Pops", "1"), item("PC Madagascar Vanilla Ice Cream", "1")), nil)
	assert.Equal(t, 2, r.ItemMatches)
	require.Len(t, r.Items, 2)
	// vanilla ice cream should match the ice cream, not the pops
	assert.Equal(t, "PC Madagascar Vanilla Ice Cream", r.Items[0].Detected.Name)
	// popsicles should match Ice Pops
	assert.Equal(t, "Ice Pops", r.Items[1].Detected.Name)
}

func TestScorePopsiclesMatchIcePops(t *testing.T) {
	// "popsicles" stems to "pop", "Ice Pops" stems to "pop" — should match
	r := Score("freezer", gt(gti("popsicles", 1)),
		result(item("Ice Pops", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScoreShredsCheese(t *testing.T) {
	// "shredded cheese" should match "Earth's Own Cheddar Shreds" via "shred"
	r := Score("fridge", gt(gti("shredded cheese", 1)),
		result(item("Earth's Own Cheddar Shreds", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScoreFrenchRoastCoffee(t *testing.T) {
	// "french roast coffee" should match via "coffee" token
	r := Score("pantry", gt(gti("french roast coffee", 1)),
		result(item("Le Café de Puebla Torrefaction Française Coffee", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScoreMilkMatchesOnMilk(t *testing.T) {
	// "2% milk" should match "Natrel Fine-Filtered Lactose Free Milk" via "milk"
	r := Score("fridge", gt(gti("2% milk", 1)),
		result(item("Natrel Fine-Filtered Lactose Free Milk", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
}

func TestScoreNoFalsePositiveChipToken(t *testing.T) {
	// "chip" is a stop word — "chocolate chip bar" must not match "Lay's Potato Chips"
	r := Score("pantry", gt(gti("chocolate chip bar", 1)),
		result(item("Lay's Potato Chips", "1")), nil)
	assert.Equal(t, 0, r.ItemMatches)
}

func TestScoreNoFalsePositivePackagedToken(t *testing.T) {
	// "packaged" is a stop word — "parmesan cheese" must not match "Packaged Cheese"
	// since only "cheese" is shared and that's too generic alone
	r := Score("fridge", gt(gti("parmesan cheese", 1)),
		result(item("Packaged Cheese", "1")), nil)
	assert.Equal(t, 0, r.ItemMatches)
}

func TestScoreNoFalsePositiveEarthsOwn(t *testing.T) {
	// "earths own oat milk" must not match "Earth's Own Cheddar Shreds" —
	// brand name alone (earth + own) should not be sufficient
	r := Score("fridge", gt(gti("earths own oat milk", 1)),
		result(item("Earth's Own Cheddar Shreds", "1")), nil)
	assert.Equal(t, 0, r.ItemMatches)
}

func TestScoreChickenBrothMatchesSoup(t *testing.T) {
	// "chicken broth" shares "chicken" with "Simply Campbell's Chicken Noodle Soup"
	r := Score("pantry", gt(gti("chicken broth", 1)),
		result(item("Simply Campbell's Chicken Noodle Soup", "1")), nil)
	assert.Equal(t, 1, r.ItemMatches)
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
