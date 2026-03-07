package gemini

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

const defaultBaseURL = "https://generativelanguage.googleapis.com"

// request types mirror the Gemini generateContent API structure.
type request struct {
	SystemInstruction *content  `json:"system_instruction,omitempty"`
	Contents          []content `json:"contents"`
	GenerationConfig  genConfig `json:"generation_config"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inline_data,omitempty"`
}

type inlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type genConfig struct {
	ResponseMIMEType string `json:"response_mime_type"`
}

type response struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type GeminiAnalyzer struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

func NewGeminiAnalyzer(apiKey, model string) *GeminiAnalyzer {
	return &GeminiAnalyzer{
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{},
		baseURL: defaultBaseURL,
	}
}

func (a *GeminiAnalyzer) Analyze(ctx context.Context, r io.Reader, mimeType string) (*vision.AnalysisResult, error) {
	imageData, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	body := request{
		SystemInstruction: &content{
			Parts: []part{{Text: vision.ClaudeSystemPrompt}},
		},
		Contents: []content{{
			Parts: []part{
				{
					InlineData: &inlineData{
						MimeType: mimeType,
						Data:     base64.StdEncoding.EncodeToString(imageData),
					},
				},
				{Text: vision.ClaudeUserPrompt},
			},
		}},
		GenerationConfig: genConfig{
			ResponseMIMEType: "application/json",
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", a.baseURL, a.model, a.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call gemini: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close gemini response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini returned status %d: %s", resp.StatusCode, errBody)
	}

	var respBody response
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(respBody.Candidates) == 0 || len(respBody.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini returned no candidates")
	}

	responseText := respBody.Candidates[0].Content.Parts[0].Text

	result, err := vision.ParseJSONResponse(responseText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vision response: %w", err)
	}

	if result.Status == vision.StatusUnclear {
		return nil, fmt.Errorf("image is unclear: please retake the photo")
	}

	return result, nil
}
