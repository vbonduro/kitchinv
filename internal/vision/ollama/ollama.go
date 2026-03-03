package ollama

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

type OllamaAnalyzer struct {
	host   string
	model  string
	client *http.Client
}

func NewOllamaAnalyzer(host, model string) *OllamaAnalyzer {
	return &OllamaAnalyzer{
		host:   host,
		model:  model,
		client: &http.Client{},
	}
}

func (a *OllamaAnalyzer) Analyze(ctx context.Context, r io.Reader, mimeType string) (*vision.AnalysisResult, error) {
	imageData, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(imageData)

	reqBody := map[string]interface{}{
		"model":  a.model,
		"prompt": vision.OllamaAnalysisPrompt,
		"images": []string{encoded},
		"stream": false,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.host+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call ollama: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close ollama response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var respBody struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result, err := vision.ParseJSONResponse(respBody.Response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vision response: %w", err)
	}

	if result.Status == vision.StatusUnclear {
		return nil, fmt.Errorf("image is unclear: please retake the photo")
	}

	return result, nil
}
