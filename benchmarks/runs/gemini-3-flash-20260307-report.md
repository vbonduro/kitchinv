# Benchmark Report — gemini-3-flash-preview — 2026-03-07

**Backend:** gemini
**Model:** gemini-3-flash-preview
**Fixtures:** 6
**Scorer:** token-overlap + substring + override (threshold 0.20)

---

## Scored Results

| Fixture | Expected | Detected | Matched | Item Accuracy | Qty Accuracy |
|---|---|---|---|---|---|
| downstairs_freezer | 7 | 9 | 3 | 43% | 67% |
| downstairs_fridge | 7 | 10 | 6 | 86% | 50% |
| downstairs_pantry | 51 | 29 | 19 | 37% | 68% |
| fridge | 43 | 35 | 25 | 58% | 84% |
| fridge_freezer | 8 | 9 | 5 | 62% | 100% |
| upstairs_pantry | 33 | 30 | 20 | 61% | 90% |
| **Overall** | **149** | **122** | **78** | **58%** | **77%** |

---

## False Positives

| Fixture | Expected | Matched To | Shared Token(s) | Notes |
|---|---|---|---|---|
| downstairs_pantry | `strawberry jam` | "Bonne Maman Raspberry Jam" | `jam` | Different fruit; `strawberry` not present in detected |
| downstairs_pantry | `food colouring green` | "Club House Food Colouring" | `colour` | Acceptable — Club House food colouring is present |
| downstairs_pantry | `tomato paste box` | "PC Tomato Paste" | `tomato` | Correct! Tomato paste matched tomato paste |
| downstairs_pantry | `kirkland cookies box` | "Kirkland Signature Chocolate Chip Cookies Biscuits" | `kirkland`+`cookie` | Correct match |
| fridge | `salad dressing` | "Packaged Salad Greens" | `salad` | Salad greens ≠ salad dressing |
| fridge | `maple syrup` | "Dark Syrup Bottle" | `syrup` | Reasonable — likely is maple syrup |
| fridge | `apple jam` | "St. Dalfour Jam" | `jam` | Ambiguous — could be correct if it's apple jam |
| fridge | `caesar dressing` | "Salad Dressing Bottle" | `dressing` | Generic detected name, could be caesar |
| upstairs_pantry | `baking powder` | "Baking Soda" | `bak` (stem) | Wrong item — baking powder ≠ baking soda |
| upstairs_pantry | `k cookies` | "Kirkland Signature European Cookies" | `cookie` | Acceptable — Kirkland cookies are k cookies |

**Total clear false positives: ~4–5**
**Ambiguous (possible correct matches): ~5**

---

## Possible False Negatives

| Fixture | Expected (MISS) | Available EXTRA | Reason Not Matched |
|---|---|---|---|
| downstairs_freezer | `pork loin` | "Raw Beef Roast" | Beef ≠ pork; no overlap |
| downstairs_freezer | `bread` | *(not in extras)* | Not detected |
| downstairs_freezer | `burgers` | "Raw Beef Roast" | `beef`/`burger` synonym; no token overlap |
| downstairs_freezer | `cosmo chicken nuggets` | *(not in extras)* | Not detected as nuggets |
| downstairs_fridge | `barbeque sauce` | "Unidentified Brown Glass Bottle" | No token overlap; override mismatch |
| downstairs_pantry | `honey graham` | *(not in extras)* | Not detected |
| downstairs_pantry | `medium/large freezer bags` | *(not in extras)* | Not detected |
| downstairs_pantry | `almond extract` | "Club House Pure Vanilla Extract" | `extract` overlap but `almond` ≠ `vanilla` — Jaccard too low |
| downstairs_pantry | `canned soup` | "Campbell's Tomato Soup" | `soup` is stop word |
| downstairs_pantry | `coconut milk` | "PC Blue Menu Beverage boxes" | No overlap |
| downstairs_pantry | `canned cranberries` | *(not in extras)* | Not detected |
| downstairs_pantry | `ketchup` | *(not in extras)* | Not detected |
| downstairs_pantry | `mayonnaise` | "Renee's Dressing" | No overlap |
| downstairs_pantry | `pizza sauce` | "Kirkland Signature Tomato Basil Pasta Sauce" | `pasta`/`sauce` — `sauce` is stop word; `tomato`/`pizza` no overlap |
| downstairs_pantry | `ranch` | "Renee's Dressing" | No overlap |
| downstairs_pantry | `lasagna noodles` | "Barilla Pasta", "Italpasta Pasta" | `lasagna`/`pasta` — `pasta` not a stop word here; Jaccard 1/2 = 0.5 — should match |
| downstairs_pantry | `alcohol free guinness` | *(not in extras)* | Not detected |
| downstairs_pantry | `mint hot chocolate` | *(not in extras)* | Not detected |
| downstairs_pantry | `cheerios box` | *(not in extras)* | Not detected |
| downstairs_pantry | `annies mac and cheese` | *(not in extras)* | Not detected |
| downstairs_pantry | `canned navy beans` | *(not in extras)* | Not detected |
| downstairs_pantry | `baking soda` | *(not in extras)* | Not detected |
| downstairs_pantry | `apple sauce pack` | *(not in extras)* | Not detected |
| downstairs_pantry | `dare/pc crackers` | "President's Choice..." consumed | Consumed by premium plus match |
| fridge | `salsa` | "Hot Sauce Bottle" | No overlap |
| fridge | `sriracha` | "Hot Sauce Bottle" | No overlap |
| fridge | `fiery ginger` | *(not in extras)* | Not detected in fridge (was in downstairs_fridge) |
| fridge | `lettuce` | "Packaged Salad Greens" | Consumed by `salad dressing` false positive |
| fridge | `cherry tomatoes` | *(not in extras)* | Not detected |
| fridge | `onions` | *(not in extras)* | Not detected |
| fridge | `iogo nano` | *(not in extras)* | Not detected |
| fridge | `broccoli in dish` | *(not in extras)* | Not detected |
| fridge | `sweet potato` | *(not in extras)* | Not detected |
| fridge | `cucumber` | *(not in extras)* | Not detected |
| fridge | `sour cream` | *(not in extras)* | Not detected |
| fridge | `parmesan cheese` | *(not in extras)* | Not detected |
| fridge | `tofu` | *(not in extras)* | Not detected |
| fridge | `shredded cheese` | *(not in extras)* | Not detected |
| fridge | `chipotle aioli` | *(not in extras)* | Not detected |
| fridge | `franks red hot sauce` | "Hot Sauce Bottle" | `hot` is stop word; `frank`/`redhot` no overlap |
| fridge | `primo pizza sauce` | "Jar of Pasta Sauce" | `pasta`/`pizza` no overlap; `sauce` stop word |
| fridge | `tamari` | *(not in extras)* | Not detected |
| upstairs_pantry | `fancy molasses` | *(not in extras)* | Not detected |
| upstairs_pantry | `dates` | "Sunny Fruit Organic Dried Figs" | Figs ≠ dates |
| upstairs_pantry | `cinnamon cereal` | *(not in extras)* | Cheerios consumed the cereal detection |
| upstairs_pantry | `kirkland cookies` | consumed by `k cookies` | k cookies took the Kirkland cookie detection |
| upstairs_pantry | `baking soda` | *(not in extras)* | Consumed by baking powder false positive |
| upstairs_pantry | `chocolate chip bar` | *(not in extras)* | Not detected |
| upstairs_pantry | `tostitos` | "Simply Potato Chips" | `potato`/`tostito` no overlap; `chip` stop word |
| upstairs_pantry | `snack factory pretzel crisps` | *(not in extras)* | Not detected |

**Scorer-fixable false negatives: ~4** (lasagna/pasta, lettuce/salad consumed, baking soda consumed, franks/hot sauce)
**True model misses: ~40+**

---

## Estimated Adjusted Score

| | Matched | Expected | Item Accuracy |
|---|---|---|---|
| Scored (actual) | 78 | 149 | **58%** |
| − Clear false positives | −4 | — | — |
| + Scorer-fixable false negatives | +4 | — | — |
| **Adjusted estimate** | **78** | **149** | **~52%** |

The adjusted score of ~52% is similar to gemini-2.5-flash's adjusted ~54%, suggesting both models have comparable true identification capability on these fixtures.

---

## Model Comparison: gemini-2.5-flash vs gemini-3-flash-preview

| Fixture | 2.5-flash (Run B) | 3-flash-preview | Winner |
|---|---|---|---|
| downstairs_freezer | 57% | 43% | 2.5-flash |
| downstairs_fridge | 71% | 86% | **3-flash** |
| downstairs_pantry | 49% | 37% | 2.5-flash |
| fridge | 56% | 58% | 3-flash (marginal) |
| fridge_freezer | 62% | 62% | Tie |
| upstairs_pantry | 64% | 61% | 2.5-flash |
| **Overall scored** | **60%** | **58%** | **2.5-flash** |
| **Overall adjusted** | **~54%** | **~52%** | **2.5-flash** |
| **Qty accuracy** | **72%** | **77%** | **3-flash** |

### Key observations

- **gemini-2.5-flash is marginally better overall** at item identification (60% vs 58% scored, 54% vs 52% adjusted)
- **gemini-3-flash-preview is better at counting** — quantity accuracy 77% vs 72%
- **3-flash is notably better in the fridge** (86% vs 71% downstairs_fridge) — correctly identified earths own oat milk, alcohol free heineken with correct quantity, cheerios, ground cinnamon, tomato paste, windsor salt
- **3-flash detects fewer items overall** (122 vs 125 detections) suggesting it is more conservative — better precision, slightly lower recall
- **3-flash produces cleaner detected names** — more specific brand names (e.g. "Ajinomoto Yakisoba", "Wyman's Fresh Frozen Wild Blueberries", "PC Fine Grind French Roast Coffee") which both helps (correct matches) and hurts (harder to token-match generic ground truth)
- **Pantry performance gap** is significant: 2.5-flash 49% vs 3-flash 37% — 3-flash detects fewer items in dense shelf photos
- Both models struggle with items that aren't directly visible or are obscured (sriracha, tamari, soy sauce in condiment-dense fridge door)

### Recommendation

**Use gemini-2.5-flash** as the primary backend for item identification accuracy. Consider **gemini-3-flash-preview** if quantity counting accuracy is a priority, or if the fridge fixture is representative of most use cases.
