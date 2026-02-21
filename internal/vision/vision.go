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
	// AnalyzeStream sends DetectedItems on the returned channel as they are
	// parsed from the model stream. The channel is closed when the stream ends
	// or ctx is cancelled. A non-nil error is sent as a StreamError if the
	// stream fails mid-way; callers should type-assert items to *StreamError.
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
