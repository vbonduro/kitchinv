package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vbonduro/kitchinv/internal/vision"
)

const defaultAPIURL = "https://api.anthropic.com/v1/messages"

// anthropicVersion is the Anthropic Messages API version header value.
const anthropicVersion = "2023-06-01"

// request types mirror the Anthropic Messages API structure.
type request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string  `json:"role"`
	Content []block `json:"content"`
}

type block struct {
	Type   string  `json:"type"`
	Text   string  `json:"text,omitempty"`
	Source *source `json:"source,omitempty"`
}

type source struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type response struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// streamRequest extends request with stream:true for the streaming API.
type streamRequest struct {
	request
	Stream bool `json:"stream"`
}

type ClaudeAnalyzer struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

func NewClaudeAnalyzer(apiKey, model string) *ClaudeAnalyzer {
	return &ClaudeAnalyzer{
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{},
		baseURL: defaultAPIURL,
	}
}

// buildMessages constructs the Anthropic API message payload for a vision request.
func buildMessages(imageData []byte, mimeType string) []message {
	return []message{{
		Role: "user",
		Content: []block{
			{
				Type: "image",
				Source: &source{
					Type:      "base64",
					MediaType: normaliseMIME(mimeType),
					Data:      base64.StdEncoding.EncodeToString(imageData),
				},
			},
			{Type: "text", Text: vision.AnalysisPrompt},
		},
	}}
}

// newHTTPRequest creates an authenticated POST request to the Claude API.
func (a *ClaudeAnalyzer) newHTTPRequest(ctx context.Context, payload []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	return req, nil
}

func (a *ClaudeAnalyzer) Analyze(ctx context.Context, r io.Reader, mimeType string) (*vision.AnalysisResult, error) {
	imageData, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	body := request{
		Model: a.model,
		// 1024 tokens is well above the expected response for a typical pantry photo
		// (≈30 items × ~15 tokens each = ~450 tokens), with headroom for verbose models.
		MaxTokens: 1024,
		Messages:  buildMessages(imageData, mimeType),
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := a.newHTTPRequest(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call claude: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close claude response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("claude returned status %d: %s", resp.StatusCode, errBody)
	}

	var respBody response
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var responseText string
	for _, blk := range respBody.Content {
		if blk.Type == "text" {
			responseText = blk.Text
			break
		}
	}

	return &vision.AnalysisResult{
		Items:       vision.ParseResponse(responseText),
		RawResponse: responseText,
	}, nil
}

// AnalyzeStream implements vision.StreamAnalyzer using the Anthropic streaming
// Messages API. It sends stream:true and parses SSE events, emitting a
// DetectedItem on the channel each time a complete "name | qty | notes" line
// is accumulated from text_delta events.
func (a *ClaudeAnalyzer) AnalyzeStream(ctx context.Context, r io.Reader, mimeType string) (<-chan vision.StreamEvent, error) {
	imageData, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	body := streamRequest{
		Stream: true,
		request: request{
			Model:     a.model,
			MaxTokens: 1024,
			Messages:  buildMessages(imageData, mimeType),
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := a.newHTTPRequest(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call claude: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("claude returned status %d: %s", resp.StatusCode, errBody)
	}

	// Buffer of 16 prevents the goroutine from blocking between item emissions
	// while the caller is processing; sized for a typical pantry photo (≈30 items).
	ch := make(chan vision.StreamEvent, 16)

	go func() {
		defer close(ch)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Error("failed to close claude stream body", "error", err)
			}
		}()

		var lineBuf strings.Builder
		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			if ctx.Err() != nil {
				return
			}

			line := scanner.Text()

			// SSE data lines start with "data: "
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := line[6:]
			if data == "[DONE]" {
				break
			}

			var event struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			if event.Type != "content_block_delta" || event.Delta.Type != "text_delta" {
				continue
			}

			// Accumulate tokens, emit an item per complete line.
			for _, c := range event.Delta.Text {
				if c == '\n' {
					line := strings.TrimSpace(lineBuf.String())
					lineBuf.Reset()
					if item := vision.ParseLine(line); item != nil {
						ch <- vision.StreamEvent{Item: item}
					}
				} else {
					lineBuf.WriteRune(c)
				}
			}
		}

		// Flush any trailing line.
		if tail := strings.TrimSpace(lineBuf.String()); tail != "" {
			if item := vision.ParseLine(tail); item != nil {
				ch <- vision.StreamEvent{Item: item}
			}
		}

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			ch <- vision.StreamEvent{Err: fmt.Errorf("read claude stream: %w", err)}
		}
	}()

	return ch, nil
}

// normaliseMIME maps browser MIME types to the values the Anthropic API accepts.
// The Anthropic API accepts only jpeg, png, gif, and webp. Unknown types are
// coerced to jpeg as the most universally supported lossy fallback. Callers
// should validate MIME types before reaching this layer.
func normaliseMIME(mimeType string) string {
	switch mimeType {
	case "image/png", "image/gif", "image/webp":
		return mimeType
	default:
		return "image/jpeg"
	}
}
