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

// StreamAnalyzer is an optional extension of VisionAnalyzer that can stream
// detected items incrementally as the model produces output.
type StreamAnalyzer interface {
	VisionAnalyzer
	// AnalyzeStream sends StreamEvents on the returned channel as the model
	// produces output. The channel is closed when the stream ends or ctx is
	// cancelled. If the stream fails mid-way, a StreamEvent with a non-nil Err
	// field is sent before the channel is closed.
	AnalyzeStream(ctx context.Context, r io.Reader, mimeType string) (<-chan StreamEvent, error)
}

// StreamEvent is either a DetectedItem or an error emitted during streaming.
type StreamEvent struct {
	Item *DetectedItem
	Err  error
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
