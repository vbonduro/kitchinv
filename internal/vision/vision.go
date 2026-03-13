package vision

import (
	"context"
	"io"
)

// OllamaAnalysisPrompt is a compact example-based prompt for smaller local models.
// Uses a concrete JSON example rather than a formal schema to minimise token usage
// while still guiding the model toward structured output.
const OllamaAnalysisPrompt = `List every food item visible in this photo.
Respond with JSON only, exactly matching this shape (no prose, no code fences):
{"status":"ok","items":[{"name":"Milk","quantity":2,"notes":"top shelf left"}]}

status must be one of: "ok" (items found), "no_items" (nothing identifiable), "not_food" (not a food area), "unclear" (image unreadable).
If status is not "ok", set items to [].`

// ClaudeSystemPrompt is placed in the system turn for Claude API calls.
// It is cached by Anthropic prompt caching after the first request, so the
// schema tokens cost ~10% of normal input price on subsequent calls.
const ClaudeSystemPrompt = `You analyse food storage area photos and return structured JSON.

For each distinct food product you can identify, provide:
- name: the food product name (e.g. "Whole Milk", "Cheddar Cheese", "Orange Juice")
- quantity: your best-estimate count of how many of this item are visible (e.g. 1, 2, 6). Must be a whole number. Never null.
- bbox: normalized bounding box [x1, y1, x2, y2] where 0,0 is top-left and 1,1 is bottom-right. Enclose the item as tightly as possible.

Respond with JSON that validates against this schema — no prose, no code fences:
{
  "required": ["status", "items"],
  "properties": {
    "status": { "enum": ["ok", "no_items", "not_food", "unclear"] },
    "items": {
      "type": "array",
      "items": {
        "required": ["name"],
        "properties": {
          "name":     { "type": "string" },
          "quantity": { "type": "integer", "minimum": 1 },
          "bbox":     { "type": "array", "items": { "type": "number", "minimum": 0, "maximum": 1 }, "minItems": 4, "maxItems": 4 }
        }
      }
    }
  }
}

Status meanings:
- ok       : one or more food items found; populate items array
- no_items : valid food storage area but nothing identifiable
- not_food : image is not a food storage area
- unclear  : image is too blurry, dark, or otherwise unreadable`

// ClaudeUserPrompt is the short user-turn message sent alongside the image.
const ClaudeUserPrompt = `List every food item visible in this photo. Be as specific as possible — include brand names where visible (e.g. "Natrel Whole Milk" not "Milk", "Kraft Peanut Butter" not "Peanut Butter"). List every individual item you can see, do not group or summarise.`

// GeminiSystemPrompt is the system instruction for Gemini API calls.
// It is tuned for Gemini's known miss patterns: grouping, freezer inference,
// condiment bottles, and non-food items in food storage areas.
const GeminiSystemPrompt = `You analyse food storage area photos and return structured JSON.

For each distinct product you can identify, provide:
- name: the product name, as specific as possible including brand (e.g. "Natrel Whole Milk", "Kraft Smooth Peanut Butter", "Sriracha Hot Sauce")
- quantity: your best-estimate count of how many of this item are visible (e.g. 1, 2, 6). Must be a whole number. Never null.
- bbox: bounding box [y1, x1, y2, x2] as integers in a 1000×1000 coordinate grid, where [0,0] is the top-left pixel and [999,999] is the bottom-right pixel. Enclose the item as tightly as possible.

Scanning rules:
- Scan every shelf and door compartment methodically, shelf by shelf, left to right, top to bottom.
- List EVERY individual product as a separate item — never group or summarise (e.g. "food colouring red", "food colouring blue", "food colouring green" as three separate items, never "Assorted food colourings").
- Include non-food items found in food storage areas: freezer bags (small/medium/large), compostable bags, paper towels, etc.
- For condiment bottles on door shelves, read each label individually: sriracha, tamari, soy sauce, maple syrup, aioli, ranch, salsa, BBQ sauce — list each one separately.
- For freezer items, infer the contents from packaging shape and any visible label text (e.g. a bread-loaf-shaped bag → "bread"); do not write "bag with unknown contents" or "frozen item".

Respond with JSON that validates against this schema — no prose, no code fences:
{
  "required": ["status", "items"],
  "properties": {
    "status": { "enum": ["ok", "no_items", "not_food", "unclear"] },
    "items": {
      "type": "array",
      "items": {
        "required": ["name"],
        "properties": {
          "name":     { "type": "string" },
          "quantity": { "type": "integer", "minimum": 1 },
          "bbox":     { "type": "array", "items": { "type": "integer", "minimum": 0, "maximum": 999 }, "minItems": 4, "maxItems": 4 }
        }
      }
    }
  }
}

Status meanings:
- ok       : one or more items found; populate items array
- no_items : valid food storage area but nothing identifiable
- not_food : image is not a food storage area
- unclear  : image is too blurry, dark, or otherwise unreadable`

// GeminiUserPrompt is the user-turn message for Gemini API calls.
const GeminiUserPrompt = `Scan every shelf and door compartment methodically, left to right, top to bottom. List EVERY individual product as a separate item — do not group or summarise. Include non-food items (bags, paper towels). For condiment bottles, read each label individually. For freezer items, describe the contents not the packaging.`

// AnalysisStatus represents the outcome of a vision analysis.
type AnalysisStatus string

const (
	StatusOK      AnalysisStatus = "ok"
	StatusNoItems AnalysisStatus = "no_items"
	StatusNotFood AnalysisStatus = "not_food"
	StatusUnclear AnalysisStatus = "unclear"
)

type VisionAnalyzer interface {
	Analyze(ctx context.Context, r io.Reader, mimeType string) (*AnalysisResult, error)
}

type AnalysisResult struct {
	Status      AnalysisStatus
	Items       []DetectedItem
	RawResponse string
}

type DetectedItem struct {
	Name     string
	Quantity string
	Notes    string
	BBox     *[4]float64 // normalized [x1, y1, x2, y2], nil if not provided
}
