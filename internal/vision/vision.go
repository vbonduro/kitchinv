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
{"status":"ok","items":[{"name":"Milk","quantity":"2 litres","notes":"opened"}]}

status must be one of: "ok" (items found), "no_items" (nothing identifiable), "not_food" (not a food area), "unclear" (image unreadable).
If status is not "ok", set items to [].`

// ClaudeSystemPrompt is placed in the system turn for Claude API calls.
// It is cached by Anthropic prompt caching after the first request, so the
// schema tokens cost ~10% of normal input price on subsequent calls.
const ClaudeSystemPrompt = `You analyse food storage area photos and return structured JSON.

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
          "quantity": { "type": ["string", "null"] },
          "notes":    { "type": ["string", "null"] }
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
const ClaudeUserPrompt = `List every food item visible in this photo.`

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
}
