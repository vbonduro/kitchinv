package claude

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

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
