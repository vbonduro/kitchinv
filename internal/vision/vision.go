package vision

import (
	"context"
	"io"
)

// AnalysisPrompt is the shared prompt used by all vision adapters.
const AnalysisPrompt = `List every food item you can see in this refrigerator/freezer/pantry photo.
For each item provide: name, approximate quantity, and any relevant notes
(e.g. opened, expired). Respond in plain text, one item per line,
format: name | quantity | notes`

type VisionAnalyzer interface {
	Analyze(ctx context.Context, r io.Reader, mimeType string) (*AnalysisResult, error)
}

type AnalysisResult struct {
	Items       []DetectedItem
	RawResponse string
}

type DetectedItem struct {
	Name     string
	Quantity string
	Notes    string
}
