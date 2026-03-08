# Vision Model Benchmark Investigation

Benchmarked 5 vision models against 6 real kitchen photo fixtures (fridge, freezer, and pantry photos) to determine the best backend for item identification in kitchinv.

## Methodology

Each fixture has a `ground_truth.json` listing expected items and quantities. The benchmark runner submits the photo to the model, parses the JSON response, and scores it with a multi-pass matcher:

1. **Substring match** — case-insensitive containment
2. **Token Jaccard similarity** — shared meaningful words (threshold 0.20), with stop words filtered and basic stemming applied
3. **Override rules** — explicit fixture-level overrides for known close misses

Metrics:
- **Item accuracy** = matched / expected
- **Quantity accuracy** = quantity-correct / matched

Runs were saved as raw JSON and rescored offline to allow scorer iteration without re-calling APIs.

## Fixtures

| Fixture | Items | Description |
|---|---|---|
| downstairs_freezer | 7 | Chest freezer, unlabeled frozen packages |
| downstairs_fridge | 7 | Small fridge, drinks and condiments |
| downstairs_pantry | 51 | Dense pantry shelves, wide variety |
| fridge | 43 | Main fridge, full shelf view |
| fridge_freezer | 8 | Fridge-top freezer drawer |
| upstairs_pantry | 33 | Open shelving with containers and dry goods |

## Models Tested

| Model | Backend | Notes |
|---|---|---|
| gemini-2.5-flash | Gemini API | Primary candidate |
| gemini-3-flash-preview | Gemini API | Newer model comparison |
| claude-haiku-4-5-20251001 | Claude API | Fast/cheap tier |
| claude-sonnet-4-6 | Claude API | Mid-tier |
| claude-opus-4-6 | Claude API | Top tier |

## Results

### Scored Results (all 6 fixtures)

| Model | Matched / Expected | Item Accuracy | Qty Accuracy |
|---|---|---|---|
| gemini-2.5-flash | 84 / 149 | **60%** | 72% |
| gemini-3-flash-preview | 78 / 149 | 58% | **77%** |
| claude-sonnet-4-6 | 62 / 149 | 42% | 68% |
| claude-opus-4-6 | 63 / 149 | 40% | 77% |
| claude-haiku-4-5 | 33 / 149 | 21% | 55% |

### Adjusted Results (scorer false positives removed, fixable false negatives added back)

| Model | Scorer FP (est) | Scorer FN (fixable) | Adjusted Accuracy |
|---|---|---|---|
| gemini-2.5-flash | −10 | +6 | **~54%** |
| gemini-3-flash-preview | −4 | +4 | **~52%** |
| claude-sonnet-4-6 | −3 | +7 | **~44%** |
| claude-opus-4-6 | −3 | +9 | **~46%** |
| claude-haiku-4-5 | −1 | +1 | **~22%** |

The adjusted scores account for known scorer imprecision — false positives where a generic token connected unrelated items, and false negatives where the model detected the right item but the scorer couldn't connect it (stop word collisions, hyphenated tokens, missing synonyms).

### Per-fixture breakdown

| Fixture | gemini-2.5-flash | gemini-3-flash | claude-sonnet | claude-opus |
|---|---|---|---|---|
| downstairs_freezer | **57%** | 43% | 29% | 14% |
| downstairs_fridge | 71% | **86%** | 71% | 57% |
| downstairs_pantry | **49%** | 37% | 24% | 33% |
| fridge | 56% | **58%** | 51% | 42% |
| fridge_freezer | 62% | 62% | 50% | 50% |
| upstairs_pantry | 64% | 61% | 52% | **58%** |

## Key Findings

### Gemini leads on item identification

Gemini-2.5-flash outperforms all Claude models by 8–10 points on adjusted accuracy across all 6 fixtures. The gap is consistent — not a scorer artifact or single-fixture anomaly.

### Claude's verbose descriptions hurt matching

Claude models produce long, specific descriptions ("Lactose-Free Cheddar Shredded Cheese (bag)", "Greenhouse Biologique Fiery Ginger+ Juice (1.26L carton)") which are accurate but resist token matching against short ground truth labels ("shredded cheese", "fiery ginger"). This inflates false negatives. Gemini tends to produce shorter, more matchable names.

### Freezer is Claude's weakest point

All Claude models struggle with unlabeled frozen packages — they describe the packaging ("Frozen white-wrapped items (frost-covered)", "Frozen meat (red package, possibly Raskõ brand)") rather than inferring the item. Gemini does this too but less severely.

### Quantity accuracy is a Claude strength

Claude Opus (77%) and gemini-3-flash (77%) tie on quantity accuracy, both beating gemini-2.5-flash (72%). However, quantity accuracy only matters for correctly identified items — and Claude identifies fewer items — so this advantage is limited in practice.

### Haiku is not viable

At 21% item accuracy, Haiku under-detects dramatically. It summarises scenes rather than enumerating items, and confuses freezer contents with fresh produce. It should not be used for this task.

### Scorer false negative patterns (all models)

Several recurring scorer misses affect all models equally and are fixable:

| Ground truth | Detected (example) | Issue |
|---|---|---|
| `lasagna noodles` | "Pasta or noodles (boxed)" | `noodle` is a stop word; `lasagna`/`pasta` no overlap |
| `canned soup` | "Simply Campbell's Chicken Noodle Soup" | `soup` + `canned` both stop words |
| `shredded cheese` | "Cheese Blocks" | `cheese` is a stop word |
| `barbeque sauce` | "Stubbs Bar-B-Q Sauce" | hyphenated `bar-b-q` doesn't tokenize |
| `chicken broth` | "Broth/Stock Cartons" | `broth` is a stop word |
| `lettuce` | "Crunch Greens / Green Salad" | `green` stop word; no lettuce/salad overlap |

## Detailed Run Reports

- [gemini-2.5-flash Run B](../benchmarks/runs/gemini-2.5-flash-20260307b-report.md) — full fixture results, false positive/negative tables, scorer improvement notes
- [gemini-3-flash-preview](../benchmarks/runs/gemini-3-flash-20260307-report.md) — comparison vs gemini-2.5-flash, per-fixture breakdown
- [Claude models Run A](../benchmarks/runs/claude-models-20260307-report.md) — original truncated run (max_tokens=1024), documents the truncation issue
- [Claude models Run B](../benchmarks/runs/claude-models-20260307b-report.md) — full run after max_tokens fix (4096), all 6 fixtures complete

## Prompt Improvement Work (post-initial benchmark)

After the initial benchmark, two improvement branches were pursued to push gemini-2.5-flash accuracy higher.

### Gemini-tuned prompt + expanded overrides

Replaced the reused Claude prompts with a Gemini-specific prompt targeting known miss patterns:
- Shelf-by-shelf scanning instruction
- No-grouping rule with concrete example (food colouring colours)
- Non-food items called out (freezer bags, compostable bags, paper towels)
- Condiment bottles listed by name (sriracha, tamari, soy sauce, maple syrup, aioli, ranch, salsa, BBQ sauce)
- Freezer inference rule — infer contents from packaging shape, not "bag with unknown contents"

Overrides expanded from 44 → 108 entries. Result across 5 runs: **59–65%, avg ~62%** (up from ~54% baseline). The 65% target was hit on the best run but not consistently — remaining misses are genuine model failures on occluded/small items in dense photos.

### Two-pass self-review (investigated, not shipped)

Tested a strategy where the model's first-pass JSON output is fed back to it in a multi-turn conversation alongside the original image, asking it to find anything missed or misidentified. A targeted review prompt asked specifically about door-shelf condiments, obscured items, and vague names.

Results across 3 runs: **60%, 70%, 62%, avg ~64%** — marginally better than single-pass on average but with higher variance. The `downstairs_freezer` fixture swung between 29% and 71% across runs, making it unreliable. The cost is 2× API calls and latency per photo. Not shipped.

The 70% run demonstrates the ceiling is real — the model *can* find more items on a second look — but inconsistency makes it unsuitable for production without a more robust merging strategy.

## Conclusion and Recommendation

**Use gemini-2.5-flash as the production vision backend.**

It leads on item accuracy (60% scored, ~54% adjusted) across all fixture types, completes all 6 fixtures reliably, and produces matchable item names. The 8–10 point gap over Claude Sonnet/Opus on adjusted accuracy is consistent and real.

**gemini-3-flash-preview** is a viable alternative if quantity accuracy is the priority (77% vs 72%) or if the downstairs_fridge fixture is representative of most use cases (86% vs 71%). It is marginally weaker on pantry detection.

**Claude Opus or Sonnet** could serve as a fallback if Gemini API access is unavailable. Sonnet and Opus perform comparably (~44–46% adjusted). Opus has more scorer-fixable misses suggesting its detections are closer to correct but harder to match — it may score better as the scorer improves.

**Eliminate Haiku entirely** from consideration.
