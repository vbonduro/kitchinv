package claude

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClaudeAnalyzeReadError(t *testing.T) {
	analyzer := NewClaudeAnalyzer("sk-test", "claude-opus-4-6")

	// Create a reader that fails
	failReader := &io.LimitedReader{R: bytes.NewReader([]byte{0xFF}), N: 0}
	_, err := analyzer.Analyze(context.Background(), failReader, "image/jpeg")

	assert.Error(t, err)
}

// Note: Full integration tests with actual API calls would require mocking
// or using httptest with proper response handling. For now we test the error path.
