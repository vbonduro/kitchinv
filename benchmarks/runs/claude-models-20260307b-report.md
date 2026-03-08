# Benchmark Report — Claude Models — 2026-03-07 (Run B, max_tokens=4096)

**Backend:** claude
**Models tested:** claude-haiku-4-5-20251001, claude-sonnet-4-6, claude-opus-4-6
**Fixtures:** 6/6 completed for all models (truncation fixed)
**Scorer:** token-overlap + substring + override (threshold 0.20)
**Change from Run A:** `max_tokens` bumped from 1024 → 4096

---

## Scored Results

### claude-haiku-4-5 (6/6 fixtures)

| Fixture | Expected | Detected | Matched | Item Accuracy | Qty Accuracy |
|---|---|---|---|---|---|
| downstairs_freezer | 7 | 5 | 0 | 0% | 0% |
| downstairs_fridge | 7 | 7 | 2 | 29% | 100% |
| downstairs_pantry | 51 | 25 | 8 | 16% | 50% |
| fridge | 43 | 21 | 12 | 28% | 83% |
| fridge_freezer | 8 | 10 | 2 | 25% | 50% |
| upstairs_pantry | 33 | 18 | 9 | 27% | 44% |
| **Overall (6 fixtures)** | **149** | **86** | **33** | **21%** | **55%** |

### claude-sonnet-4-6 (6/6 fixtures)

| Fixture | Expected | Detected | Matched | Item Accuracy | Qty Accuracy |
|---|---|---|---|---|---|
| downstairs_freezer | 7 | 14 | 2 | 29% | 50% |
| downstairs_fridge | 7 | 8 | 5 | 71% | 40% |
| downstairs_pantry | 51 | 30 | 12 | 24% | 83% |
| fridge | 43 | 36 | 22 | 51% | 91% |
| fridge_freezer | 8 | 10 | 4 | 50% | 75% |
| upstairs_pantry | 33 | 30 | 17 | 52% | 71% |
| **Overall (6 fixtures)** | **149** | **128** | **62** | **42%** | **68%** |

### claude-opus-4-6 (6/6 fixtures)

| Fixture | Expected | Detected | Matched | Item Accuracy | Qty Accuracy |
|---|---|---|---|---|---|
| downstairs_freezer | 7 | 13 | 1 | 14% | 100% |
| downstairs_fridge | 7 | 8 | 4 | 57% | 25% |
| downstairs_pantry | 51 | 29 | 17 | 33% | 82% |
| fridge | 43 | 32 | 18 | 42% | 89% |
| fridge_freezer | 8 | 9 | 4 | 50% | 75% |
| upstairs_pantry | 33 | 30 | 19 | 58% | 89% |
| **Overall (6 fixtures)** | **149** | **121** | **63** | **40%** | **77%** |

---

## Full Model Comparison (all 6 fixtures)

| Model | Matched / Expected | Item Accuracy | Qty Accuracy |
|---|---|---|---|
| gemini-2.5-flash (Run B) | 84 / 149 | **60%** | 72% |
| gemini-3-flash-preview | 78 / 149 | 58% | **77%** |
| claude-sonnet-4-6 | 62 / 149 | 42% | 68% |
| claude-opus-4-6 | 63 / 149 | 40% | 77% |
| claude-haiku-4-5 | 33 / 149 | 21% | 55% |

---

## Notable Observations Per Model

### claude-haiku-4-5
Still fundamentally under-detects — only 86 detected items vs 149 expected. The same pattern from Run A: detects in broad category terms ("Canned Vegetables or Beans", "Frozen Vegetables"), confuses freezer contents (sees fresh lettuce and ground beef instead of frozen items), and summarises dense shelves rather than enumerating. **Not viable.**

### claude-sonnet-4-6
Significant improvement on large fixtures now that truncation is fixed — pantry jumped from SKIPPED to 24% and upstairs_pantry to 52%. The fridge fixture at 51% is comparable to gemini. Sonnet is verbose and specific with brand names (Hellmann's, Heineken, Liberté, Welch's) which helps token matching but also generates many extras. Quantity accuracy improved (68%) vs Run A (58%).

### claude-opus-4-6
Upstairs_pantry at 58% is opus's best fixture. Opus identifies very specific items (KitchWise Brown Sugar, PC Buttermilk Pancake Mix, Cora Bread Crumbs, César Légère Dressing) with high confidence. The freezer (14%) is opus's weak point — it saw 13 detailed frozen meat descriptions but matched only 1. Quantity accuracy is excellent at 77%, matching gemini-3-flash.

---

## Comparable Fixtures: All 5 Models on Same 6 Fixtures

| Model | Item Accuracy | Qty Accuracy |
|---|---|---|
| gemini-2.5-flash | **60%** | 72% |
| gemini-3-flash-preview | 58% | **77%** |
| claude-sonnet-4-6 | 42% | 68% |
| claude-opus-4-6 | 40% | 77% |
| claude-haiku-4-5 | 21% | 55% |

Gemini models clearly outperform Claude on full-fixture item accuracy. The gap is real — not a truncation artifact.

---

## Notable Misses and False Positives

### Consistent misses across all Claude models
- `downstairs_freezer`: raisin bread, pork loin, bread, burgers, cosmo chicken nuggets — Claude sees the freezer as packed with generic frozen meat packages, missing the specific items entirely
- `downstairs_pantry`: coconut milk, canned cranberries, pizza sauce, ranch, large/medium freezer bags, compost bags — these items are simply not detected
- `fridge`: maple syrup, sriracha (haiku/sonnet), cherry tomatoes, onions, iogo nano, sour cream, tamari — items in the fridge door or lower shelves
- `fridge_freezer`: butterscotch ice cream, hotdog buns, blueberries — all three models miss these consistently

### Interesting false positives
- **Haiku**: "Natrel Milk" matched `oat milk`; "Earth's Own Oat Beverage" matched `protein oats` — aggressive token matching on generic descriptions
- **Sonnet**: "Cooked Meat / Protein (brown)" matched `protein oats` via `protein` token; "Mixed Greens / Salad" matched `salad dressing` via `salad` substring
- **Opus**: "Strawberries" matched `strawberry jam` via `strawberr` stem (strawberries consumed the wrong slot); "Ground Coffee Beans" matched `ground cinnamon` via `ground` token

### Scorer-fixable misses
- `earths own oat milk` still missed by all three models — the model detects "Earth's Own Plant-Based Beverage" (opus) but `earth`+`own` are stop words
- `barbeque sauce` — detected as "Sauce/Condiment Bottle" (sonnet) with no override hit; override targets "Sauce Bottle" but name doesn't match exactly
- `downstairs_freezer` items (raisin bread, burgers) — model describes unlabeled frozen packages generically; these are true model misses, not scorer issues

---

## Recommendations

1. **gemini-2.5-flash remains the recommended production backend** — 60% vs 40-42% for Claude Sonnet/Opus on full fixtures
2. **Claude Haiku eliminated** — 21% is not viable
3. **Claude Sonnet/Opus are usable but significantly behind Gemini** — the gap on pantry fixtures is particularly large (Sonnet 24%, Opus 33% vs Gemini 49-58%)
4. **Freezer performance** is a weakness for all Claude models — generic frozen package descriptions prevent specific item matching
5. **Quantity accuracy parity**: Opus (77%) and gemini-3-flash (77%) tie; this is a Claude strength relative to item accuracy
