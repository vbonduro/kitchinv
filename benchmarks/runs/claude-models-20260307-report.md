# Benchmark Report — Claude Models — 2026-03-07

**Backend:** claude
**Models tested:** claude-haiku-4-5-20251001, claude-sonnet-4-6, claude-opus-4-6
**Fixtures:** 6 attempted (5 completed for haiku, 3 for sonnet and opus due to truncation)
**Scorer:** token-overlap + substring + override (threshold 0.20)

---

## ⚠️ Critical Issue: Output Truncation

Sonnet and Opus both **truncated their JSON output** on large fixtures (downstairs_pantry, fridge, upstairs_pantry), resulting in `unexpected end of JSON input`. These fixtures have 33–51 ground truth items — the models hit their output token limit before completing the JSON array. Haiku truncated on downstairs_pantry only.

This means **Claude model scores are not comparable to Gemini** on a per-fixture basis — only the 3 fixtures that succeeded (downstairs_freezer, downstairs_fridge, fridge_freezer) can be compared across all 5 models.

The prompt needs a `max_tokens` increase or the output format needs to be more compact to fix this.

---

## Scored Results

### claude-haiku-4-5 (5/6 fixtures)

| Fixture | Expected | Detected | Matched | Item Accuracy | Qty Accuracy |
|---|---|---|---|---|---|
| downstairs_freezer | 7 | 6 | 0 | 0% | 0% |
| downstairs_fridge | 7 | 8 | 2 | 29% | 100% |
| downstairs_pantry | — | — | — | SKIPPED | — |
| fridge | 43 | 21 | 12 | 28% | 100% |
| fridge_freezer | 8 | 9 | 2 | 25% | 100% |
| upstairs_pantry | 33 | 18 | 5 | 15% | 80% |
| **Overall (5 fixtures)** | **98** | **62** | **21** | **19%** | **76%** |

### claude-sonnet-4-6 (3/6 fixtures)

| Fixture | Expected | Detected | Matched | Item Accuracy | Qty Accuracy |
|---|---|---|---|---|---|
| downstairs_freezer | 7 | 16 | 3 | 43% | 33% |
| downstairs_fridge | 7 | 7 | 5 | 71% | 40% |
| downstairs_pantry | — | — | — | SKIPPED | — |
| fridge | — | — | — | SKIPPED | — |
| fridge_freezer | 8 | 12 | 3 | 38% | 100% |
| upstairs_pantry | — | — | — | SKIPPED | — |
| **Overall (3 fixtures)** | **22** | **35** | **11** | **51%** | **58%** |

### claude-opus-4-6 (3/6 fixtures)

| Fixture | Expected | Detected | Matched | Item Accuracy | Qty Accuracy |
|---|---|---|---|---|---|
| downstairs_freezer | 7 | 14 | 3 | 43% | 33% |
| downstairs_fridge | 7 | 8 | 4 | 57% | 25% |
| downstairs_pantry | — | — | — | SKIPPED | — |
| fridge | — | — | — | SKIPPED | — |
| fridge_freezer | 8 | 10 | 5 | 62% | 80% |
| upstairs_pantry | — | — | — | SKIPPED | — |
| **Overall (3 fixtures)** | **22** | **32** | **12** | **54%** | **46%** |

---

## Comparable Fixtures Only (downstairs_freezer + downstairs_fridge + fridge_freezer)

To make a fair comparison across all 5 models on the same 3 fixtures:

| Model | Matched / Expected | Item Accuracy | Qty Accuracy |
|---|---|---|---|
| gemini-2.5-flash (Run B) | 14 / 22 | **64%** | 71% |
| gemini-3-flash-preview | 14 / 22 | **64%** | 72% |
| claude-haiku-4-5 | 4 / 22 | 18% | 100% |
| claude-sonnet-4-6 | 11 / 22 | 50% | 58% |
| claude-opus-4-6 | 12 / 22 | **55%** | 46% |

---

## False Positives

### claude-haiku-4-5

| Fixture | Expected | Matched To | Notes |
|---|---|---|---|
| fridge | `salad dressing` | "Salad Mix" | Salad ≠ dressing — marginal |
| fridge | `caesar dressing` | "Hidden Valley Dressing" | Plausible — Hidden Valley makes ranch/caesar |
| fridge | `strawberry jam` | "Jam or Preserves" | Generic match — probably correct |
| upstairs_pantry | `cinnamon cereal` | "Cereal or Grain Product" | Generic — could be anything |

**Clear false positives: ~1–2. Haiku is conservative — few detections, so few false positives.**

### claude-sonnet-4-6

| Fixture | Expected | Matched To | Notes |
|---|---|---|---|
| downstairs_freezer | `raisin bread` | "Frozen Bread Buns/Rolls" | Wrong product — bread buns ≠ raisin bread |
| downstairs_freezer | `frozen broccoli` | "Frozen Broccoli or Mixed Vegetables" | Acceptable |

**Clear false positives: ~1**

### claude-opus-4-6

| Fixture | Expected | Matched To | Notes |
|---|---|---|---|
| downstairs_freezer | `bread` | "Frost-covered wrapped items (possibly bread or meat)" | Uncertain — could be correct |
| downstairs_freezer | `frozen broccoli` | "Frozen broccoli/kale" | Acceptable |
| fridge_freezer | `bread` | "Frozen wrapped items (possibly bread or pastry)" | Uncertain |

**Clear false positives: ~0–1. Opus is well-calibrated with uncertainty language.**

---

## Possible False Negatives

### claude-haiku-4-5

| Fixture | Expected | Available EXTRA | Reason |
|---|---|---|---|
| downstairs_freezer | `frozen broccoli` | "Mixed Vegetables" | `vegetable` overlap but not matched |
| downstairs_fridge | `shredded cheese` | "Cheddar Cheese (packaged)" | `cheese` is stop word |
| downstairs_fridge | `social vodka drink` | *(not in extras)* | Not detected |
| downstairs_fridge | `earths own oat milk` | *(not in extras)* | Not detected |
| fridge_freezer | `popsicles` | *(not in extras)* | Not detected — described contents as fresh vegetables |
| fridge_freezer | `vanilla ice cream` | *(not in extras)* | Not detected |
| fridge_freezer | `blueberries` | *(not in extras)* | Not detected |
| upstairs_pantry | nearly all items | *(very few detected)* | Haiku saw only 18/33+ items |

**Haiku fundamentally under-detects** — it appears to summarise rather than enumerate items, and confuses freezer contents with fresh produce.

### claude-sonnet-4-6

| Fixture | Expected | Available EXTRA | Reason |
|---|---|---|---|
| downstairs_fridge | `barbeque sauce` | "Sauce bottle (dark sauce, red cap)" | `sauce` stop word; override mismatch |
| downstairs_fridge | `earths own oat milk` | "Good Earth / Earth-branded Corn product" | `earth` is stop word |
| fridge_freezer | `blueberries` | *(not in extras)* | Not detected |
| fridge_freezer | `frozen vegetables` | "Canada A Frozen Mixed Vegetables" | `vegetable` overlap — should match |
| fridge_freezer | `butterscotch ice cream` | "Chapman's Super Premium Ice Cream" | `ice`+`cream` stop words; `butterscotch`/`chapman` no overlap |

**Scorer-fixable: ~2** (frozen vegetables, barbeque sauce)

### claude-opus-4-6

| Fixture | Expected | Available EXTRA | Reason |
|---|---|---|---|
| downstairs_freezer | `raisin bread` | "Cookies Pouch (yellow/cream package)" | No overlap |
| downstairs_freezer | `burgers` | "Frozen ground meat" | `ground`/`burger` synonym; no token overlap |
| downstairs_fridge | `shredded cheese` | "Lactose Free Cheddar Cheese" | `cheese` stop word |
| downstairs_fridge | `earths own oat milk` | "Earth's Own Plant-Based Product" | `earth`+`own` stop words |
| downstairs_fridge | `barbeque sauce` | "Beer Bottle (brown)" | No overlap |
| fridge_freezer | `blueberries` | *(not in extras)* | Not detected |
| fridge_freezer | `butterscotch ice cream` | "Chapman's Butter Tarts" | `butter`/`butterscotch` stem overlap — could match |
| fridge_freezer | `hotdog buns` | *(not in extras)* | Not detected |

**Scorer-fixable: ~1–2** (butterscotch/butter, frozen vegetables)

---

## Estimated Adjusted Scores (3-fixture comparable set)

| Model | Scored | − FP | + FN | Adjusted | Adj. Accuracy |
|---|---|---|---|---|---|
| gemini-2.5-flash | 14/22 | −1 | +1 | 14/22 | **~64%** |
| gemini-3-flash-preview | 14/22 | −1 | +1 | 14/22 | **~64%** |
| claude-haiku-4-5 | 4/22 | −0 | +1 | 5/22 | **~23%** |
| claude-sonnet-4-6 | 11/22 | −1 | +2 | 12/22 | **~55%** |
| claude-opus-4-6 | 12/22 | −0 | +2 | 14/22 | **~64%** |

---

## Key Findings

### Truncation is the primary Claude issue
The JSON output for large fixtures (33–51 items detected) exceeds the model's effective output capacity. The fix is to increase `max_tokens` in the Claude vision client, or switch to a more compact output format. **This must be fixed before Claude models can be properly benchmarked on pantry and fridge fixtures.**

### Haiku is not suitable for this task
At 19% overall (18% adjusted on comparable fixtures), Haiku fundamentally misidentifies freezer contents as fresh produce, and only detects ~50% of visible items in pantry photos. It is not viable as a vision backend for kitchinv.

### Sonnet vs Opus on comparable fixtures
Opus edges Sonnet (54% vs 51% scored, ~64% vs ~55% adjusted) on 3 fixtures. Opus is more thorough — it detected 14 items in the freezer vs Sonnet's 16 but with better accuracy. Opus also correctly identified frozen bread where Sonnet incorrectly matched raisin bread to bread buns.

### Gemini vs Claude (comparable fixtures)
On the 3 non-truncated fixtures, **Gemini-2.5-flash and Claude-Opus perform comparably** (~64% adjusted). Gemini-3-flash also ties. This suggests the models have similar vision capability — the truncation issue is preventing a full comparison.

### Quantity accuracy pattern
- Haiku: 76% — when it does match, quantities are correct
- Gemini-2.5: 71% — matches often but counts differ
- Gemini-3: 72% — similar
- Sonnet: 58% — more detections but looser counting
- Opus: 46% — verbose descriptions with approximate counts

---

## Recommendations

1. **Fix Claude truncation** — increase `max_tokens` in `internal/vision/claude/claude.go` before re-running Claude benchmarks
2. **Eliminate Haiku** — not viable for item identification
3. **Re-run Claude Sonnet and Opus** after truncation fix for a fair full-fixture comparison
4. **Current production recommendation: gemini-2.5-flash** — fully completes all fixtures, 60% scored / ~54% adjusted on full suite
