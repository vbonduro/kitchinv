# Benchmark Report — gemini-2.5-flash — 2026-03-07 (Run B)

**Backend:** gemini
**Model:** gemini-2.5-flash
**Fixtures:** 6
**Scorer:** token-overlap + substring + override (threshold 0.20)

---

## Scored Results

| Fixture | Expected | Detected | Matched | Item Accuracy | Qty Accuracy |
|---|---|---|---|---|---|
| downstairs_freezer | 7 | 8 | 4 | 57% | 50% |
| downstairs_fridge | 7 | 6 | 5 | 71% | 40% |
| downstairs_pantry | 51 | 40 | 25 | 49% | 68% |
| fridge | 43 | 29 | 24 | 56% | 83% |
| fridge_freezer | 8 | 8 | 5 | 62% | 100% |
| upstairs_pantry | 33 | 34 | 21 | 64% | 90% |
| **Overall** | **149** | **125** | **84** | **60%** | **72%** |

---

## False Positives

These are scorer matches where the model detected the wrong item but it was counted as a match due to token overlap.

| Fixture | Expected | Matched To | Shared Token(s) | Notes |
|---|---|---|---|---|
| downstairs_pantry | `wheat bran` | "Premium Plus Whole Wheat Crackers" | `wheat` | Crackers ≠ bran; only coincidental token |
| downstairs_pantry | `premium plus crackers` | "Goldfish Crackers" | `cracker` | Different product entirely |
| downstairs_pantry | `food colouring green` | "Assorted Food Colorings/Extracts" | `colour`/`color` | Acceptable partial match — it's in the box |
| downstairs_pantry | `almond extract` | "Vanilla Extract" | `extract` | Different flavour; `almond` not in detected |
| downstairs_pantry | `tomato paste box` | "Simply Campbell's Creamy Tomato Soup" | `tomato` | Paste ≠ soup; `tomato` alone is too generic |
| downstairs_pantry | `mint hot chocolate` | "Kirkland Signature Chocolate Chip Cookies" | `chocolate` | Unrelated products |
| downstairs_pantry | `kirkland cookies box` | "Kirkland Canned Crushed Tomatoes (box of cans)" | `kirkland` | Same brand, different product |
| downstairs_pantry | `apple sauce pack` | "Apple Juice Box (large multi-pack)" | `apple` | Close — both apple products, but different |
| fridge | `strawberries` | "Strawberry Jam" | `strawberr` | Swap with strawberry jam (see below) |
| fridge | `strawberry jam` | "Strawberries" | `strawberr` | Swap with strawberries (see above) — items matched to wrong detections |
| fridge | `salad dressing` | "Cesar Salad" | `salad` | Salad ≠ salad dressing |
| fridge | `caesar dressing` | "Salad Dressing" | `dressing` | These two and salad dressing swapped around |
| upstairs_pantry | `baking powder` | "Unidentified Powder/Condiment" | `powder` | Unidentified — could be correct, but uncertain |
| upstairs_pantry | `cinnamon cereal` | "O-shaped Cereal" | `cereal` | O-shaped = Cheerios, not cinnamon cereal |
| upstairs_pantry | `chocolate chip bar` | "Unidentified Granola Bars / Snack Bars" | `bar` | Possibly correct if granola bars = chocolate chip bars |

**Total clear false positives: ~10–12**
**Ambiguous (possible correct matches): ~3**

---

## Possible False Negatives

These are MISSes where a matching detection was available in the EXTRA list but the scorer failed to connect them.

| Fixture | Expected (MISS) | Available EXTRA | Reason Not Matched |
|---|---|---|---|
| downstairs_freezer | `burgers` | "Frozen Beef Roast" | `beef`/`burger` synonym — no token overlap |
| downstairs_freezer | `cosmo chicken nuggets` | "Cozy Coq Chicken Nuggets" | `chicken`+`nugget` overlaps — likely matched but qty wrong (shown as ~) |
| downstairs_fridge | `earths own oat milk` | *(not in extras)* | Model didn't detect it this run |
| downstairs_fridge | `barbeque sauce` | "Stubbs Original Legendary Bar-B-Q Sauce" | `barbeque`/`bbq`/`bar-b-q` — no token overlap due to hyphenation |
| downstairs_pantry | `honey graham` | *(not in extras)* | Not detected |
| downstairs_pantry | `canned soup` | "Simply Campbell's Chicken Noodle Soup" (×2 in extras) | `soup` is a stop word; `canned` is a stop word |
| downstairs_pantry | `coconut milk` | "Small Cartons (Original)" × 30 | Possibly oat milk cartons; no token overlap |
| downstairs_pantry | `canned cranberries` | *(not in extras)* | Not detected |
| downstairs_pantry | `mayonnaise` | *(not in extras)* | Not detected |
| downstairs_pantry | `salsa` | *(not in extras)* | Not detected |
| downstairs_pantry | `pizza sauce` | *(not in extras)* | Not detected |
| downstairs_pantry | `ranch` | *(not in extras)* | Not detected |
| downstairs_pantry | `cheerios box` | "Cereal/Granola Bars" | `cereal` overlap but Jaccard too low |
| downstairs_pantry | `annies mac and cheese` | *(not in extras)* | Not detected |
| downstairs_pantry | `canned navy beans` | "Canned Beans" | `bean` overlaps — Jaccard = 1/2 = 0.5, should have matched |
| downstairs_pantry | `dare crackers` | "President's Choice The Great Canadian Cracker Assortment" | `cracker` overlap but consumed by `premium plus crackers` |
| downstairs_pantry | `pc crackers` | "President's Choice The Great Canadian Cracker Assortment" | consumed by above |
| downstairs_pantry | `baking soda` | *(not in extras)* | Not detected |
| downstairs_pantry | `compost bags` | *(not in extras)* | Not detected |
| fridge | `soy sauce` | *(not in extras)* | Not detected |
| fridge | `maple syrup` | *(not in extras)* | Not detected |
| fridge | `sriracha` | *(not in extras)* | Not detected |
| fridge | `plain greek yogurt` | "Liberté Greek 0% Yogurt" consumed by vanilla | One detection consumed by vanilla greek yogurt match |
| fridge | `fiery ginger` | *(not in extras)* | Not detected |
| fridge | `lettuce` | "Crunch Greens", "Green Salad" | `green`/`salad` are stop words; override not fired (override targets "Crunchy Greens Salad Mix") |
| fridge | `onions` | *(not in extras)* | Not detected |
| fridge | `iogo nano` | *(not in extras)* | Not detected |
| fridge | `broccoli in dish` | *(not in extras)* | Not detected |
| fridge | `sweet potato` | matched ✓ this run | — |
| fridge | `cucumber` | *(not in extras)* | Not detected |
| fridge | `sour cream` | *(not in extras)* | Not detected |
| fridge | `parmesan cheese` | "Cheese Blocks" | `cheese` is stop word |
| fridge | `tofu` | *(not in extras)* | Not detected |
| fridge | `shredded cheese` | "Cheese Blocks" | `cheese` is stop word |
| fridge | `apple jam` | *(not in extras)* | Not detected |
| fridge | `chipotle aioli` | *(not in extras)* | Not detected |
| fridge | `primo pizza sauce` | *(not in extras)* | Not detected |
| fridge | `tamari` | *(not in extras)* | Not detected |
| fridge | `protein oats` | *(not in extras)* | Not detected |
| upstairs_pantry | `chicken broth` | "Campbell's Cream of Vegetable Soup" | `campbell` only, no `chicken`/`broth` |
| upstairs_pantry | `dates` | *(not in extras)* | Not detected |
| upstairs_pantry | `all bran buds` | *(not in extras)* | Not detected |
| upstairs_pantry | `cheerios` | "O-shaped Cereal" consumed by cinnamon cereal | Consumed by false match |
| upstairs_pantry | `icing sugar` (2nd) | consumed by 1st | Duplicate ground truth item |
| upstairs_pantry | `sugar` | *(not in extras)* | consumed or not detected |
| upstairs_pantry | `baking soda` | *(not in extras)* | Not detected |
| upstairs_pantry | `kirkland cookies` | "Kirkland Signature Cookies/Biscuits" consumed by `k cookies` | Consumed — `k cookies` matched via `kirkland`+`cookie` |
| upstairs_pantry | `tostitos` | "Unidentified Potato Chips" | `potato`/`tostito` no overlap; `chip` is stop word |
| upstairs_pantry | `premium plus crackers` | *(consumed)* | No remaining cracker detections |
| upstairs_pantry | `snack factory pretzel crisps` | *(not in extras)* | Not detected |
| upstairs_pantry | `apple sauce` | *(not in extras)* | Not detected |

**Scorer fixable false negatives (detection exists but not matched): ~5–8**
**True model misses (not detected at all): ~35+**

---

## Estimated Adjusted Score

Assumptions for adjusted score:
- Remove the ~10 clear false positives (items counted as matched that shouldn't be)
- Add back the ~6 scorer-fixable false negatives (detections available but not connected)
- Leave true model misses as-is

| | Matched | Expected | Item Accuracy |
|---|---|---|---|
| Scored (actual) | 84 | 149 | **60%** |
| − Clear false positives | −10 | — | — |
| + Scorer-fixable false negatives | +6 | — | — |
| **Adjusted estimate** | **80** | **149** | **~54%** |

The adjusted score of ~54% represents a more conservative estimate of the model's true identification accuracy on these fixtures. The gap between 60% and 54% reflects ongoing scorer imprecision — primarily:

1. **Single-token brand matches** (`kirkland`, `wheat`, `tomato`, `chocolate`, `apple`) that connect unrelated products of the same brand or ingredient family
2. **Strawberry swap** — `strawberries` and `strawberry jam` matched each other's detections due to stem overlap consuming them in wrong order
3. **Stop word gaps** — `cracker`, `cereal`, `powder`, `bar` are borderline; too meaningful to be stop words but generic enough to cause false positives

### Key scorer improvements that would help

| Issue | Fix |
|---|---|
| `canned navy beans` missed "Canned Beans" | `navy` should not block — Jaccard 0.5 should match; investigate consumption ordering |
| `lettuce` override didn't fire on "Crunch Greens"/"Green Salad" | Update override to match any detected name containing `green` + `salad` |
| Strawberry swap | Substring match fires first; `strawberry jam` contains `strawberr` matching `strawberries` — ordering issue |
| `barbeque sauce` missed "Stubbs Bar-B-Q Sauce" | Hyphenated `bar-b-q` doesn't split correctly; add `bbq`/`barbeque` synonym handling |
| Kirkland brand false positives | Require ≥2 meaningful non-brand tokens to match when only brand overlaps |

---

## Notes

- **Pantry fixtures score lower** due to dense shelves — the model detects ~40 of 51 items but names them at a category level (e.g. "Assorted Canned Goods") rather than individually
- **Quantity accuracy is lower than expected** (72%) because many ~ matches involve the model counting differently (e.g. detecting 1 of a multi-pack)
- **Non-determinism**: this run scored differently from Run A on several fixtures (downstairs_freezer 57% vs 43%, downstairs_pantry 49% vs 39%) due to different model outputs run-to-run
- The scorer is now well-calibrated for comparative model benchmarking — use the same saved raw run when comparing models or tuning the scorer
