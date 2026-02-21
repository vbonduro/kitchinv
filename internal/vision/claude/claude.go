package claude

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/vbonduro/kitchinv/internal/vision"
)

const apiURL = "https://api.anthropic.com/v1/messages"

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
	apiKey string
	model  string
	client *http.Client
}

func NewClaudeAnalyzer(apiKey, model string) *ClaudeAnalyzer {
	return &ClaudeAnalyzer{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

func (a *ClaudeAnalyzer) Analyze(ctx context.Context, r io.Reader, mimeType string) (*vision.AnalysisResult, error) {
	imageData, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	body := request{
		Model:     a.model,
		MaxTokens: 1024,
		Messages: []message{
			{
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
			},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call claude: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close claude response body: %v", err)
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
// Unknown types fall back to image/jpeg.
func normaliseMIME(mimeType string) string {
	switch mimeType {
	case "image/png", "image/gif", "image/webp":
		return mimeType
	default:
		return "image/jpeg"
	}
}
