package ollama

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
	// Read image data
	imageData, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	// Encode image to base64
	encoded := base64.StdEncoding.EncodeToString(imageData)

	// Build request
	reqBody := map[string]interface{}{
		"model":  a.model,
		"prompt": vision.AnalysisPrompt,
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

	items := vision.ParseResponse(respBody.Response)

	return &vision.AnalysisResult{
		Items:       items,
		RawResponse: respBody.Response,
	}, nil
}

// AnalyzeStream implements vision.StreamAnalyzer. It sends stream:true to
// Ollama and parses the newline-delimited JSON response, emitting a
// DetectedItem on the channel each time a complete "name | qty | notes" line
// is accumulated from the token stream.
func (a *OllamaAnalyzer) AnalyzeStream(ctx context.Context, r io.Reader, mimeType string) (<-chan vision.StreamEvent, error) {
	imageData, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(imageData)

	reqBody := map[string]interface{}{
		"model":  a.model,
		"prompt": vision.AnalysisPrompt,
		"images": []string{encoded},
		"stream": true,
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

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	// Buffer of 16 prevents the goroutine from blocking between item emissions
	// while the caller is processing; sized for a typical pantry photo (≈30 items).
	ch := make(chan vision.StreamEvent, 16)

	go func() {
		defer close(ch)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Error("failed to close ollama stream body", "error", err)
			}
		}()

		// accumulates tokens until we have a complete line
		var lineBuf strings.Builder

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			if ctx.Err() != nil {
				return
			}

			var chunk struct {
				Response string `json:"response"`
				Done     bool   `json:"done"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
				ch <- vision.StreamEvent{Err: fmt.Errorf("parse chunk: %w", err)}
				return
			}

			// Accumulate tokens. Emit an item for each complete line.
			for _, c := range chunk.Response {
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

			if chunk.Done {
				// flush any trailing line
				line := strings.TrimSpace(lineBuf.String())
				if item := vision.ParseLine(line); item != nil {
					ch <- vision.StreamEvent{Item: item}
				}
				return
			}
		}

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			ch <- vision.StreamEvent{Err: fmt.Errorf("read stream: %w", err)}
		}
	}()

	return ch, nil
}
