// Command benchmark runs vision analysis against a set of fixture images and
// scores the results against known ground truth item lists.
//
// Usage:
//
//	benchmark -fixtures ./benchmarks/fixtures [-json]
//
// Fixture layout:
//
//	fixtures/
//	  my-fridge/
//	    image.jpg          (or .jpeg, .png, .webp)
//	    ground_truth.json  ({"items":[{"name":"Milk","quantity":2}]})
//
// The vision backend is configured via environment variables (same as the main
// server): VISION_BACKEND, CLAUDE_API_KEY, CLAUDE_MODEL, GEMINI_API_KEY,
// GEMINI_MODEL, OLLAMA_HOST, OLLAMA_MODEL.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/vbonduro/kitchinv/internal/benchmark"
	"github.com/vbonduro/kitchinv/internal/config"
	"github.com/vbonduro/kitchinv/internal/vision"
	claudevision "github.com/vbonduro/kitchinv/internal/vision/claude"
	geminivision "github.com/vbonduro/kitchinv/internal/vision/gemini"
	ollamavision "github.com/vbonduro/kitchinv/internal/vision/ollama"
)

var imageExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

// Summary is the aggregate result across all fixtures.
type Summary struct {
	Model         string                `json:"model"`
	Backend       string                `json:"backend"`
	Fixtures      int                   `json:"fixtures"`
	Results       []benchmark.MatchResult `json:"results"`
	AvgItemAccuracy     float64         `json:"avg_item_accuracy"`
	AvgQuantityAccuracy float64         `json:"avg_quantity_accuracy"`
}

func main() {
	fixturesDir := flag.String("fixtures", "benchmarks/fixtures", "path to fixtures directory")
	jsonOut := flag.Bool("json", false, "output results as JSON")
	flag.Parse()

	cfg := config.Load()

	analyzer, modelName, err := newAnalyzer(cfg)
	if err != nil {
		log.Fatalf("failed to create vision analyzer: %v", err)
	}

	entries, err := os.ReadDir(*fixturesDir)
	if err != nil {
		log.Fatalf("failed to read fixtures directory %q: %v", *fixturesDir, err)
	}

	var results []benchmark.MatchResult

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fixturePath := filepath.Join(*fixturesDir, entry.Name())

		gt, imgPath, mimeType, err := loadFixture(fixturePath)
		if err != nil {
			log.Printf("skipping %s: %v", entry.Name(), err)
			continue
		}

		f, err := os.Open(imgPath)
		if err != nil {
			log.Printf("skipping %s: failed to open image: %v", entry.Name(), err)
			continue
		}

		fmt.Fprintf(os.Stderr, "analysing %s...\n", entry.Name())
		result, err := analyzer.Analyze(context.Background(), f, mimeType)
		f.Close()
		if err != nil {
			log.Printf("skipping %s: analysis failed: %v", entry.Name(), err)
			continue
		}

		mr := benchmark.Score(entry.Name(), gt, result)
		results = append(results, mr)
	}

	if len(results) == 0 {
		log.Fatal("no fixtures processed")
	}

	summary := buildSummary(cfg.VisionBackend, modelName, results)

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(summary); err != nil {
			log.Fatalf("failed to encode JSON: %v", err)
		}
		return
	}

	printSummary(summary)
}

func newAnalyzer(cfg *config.Config) (vision.VisionAnalyzer, string, error) {
	switch cfg.VisionBackend {
	case "claude":
		if cfg.ClaudeAPIKey == "" {
			return nil, "", fmt.Errorf("CLAUDE_API_KEY must be set when VISION_BACKEND=claude")
		}
		return claudevision.NewClaudeAnalyzer(cfg.ClaudeAPIKey, cfg.ClaudeModel), cfg.ClaudeModel, nil
	case "gemini":
		if cfg.GeminiAPIKey == "" {
			return nil, "", fmt.Errorf("GEMINI_API_KEY must be set when VISION_BACKEND=gemini")
		}
		return geminivision.NewGeminiAnalyzer(cfg.GeminiAPIKey, cfg.GeminiModel), cfg.GeminiModel, nil
	default:
		return ollamavision.NewOllamaAnalyzer(cfg.OllamaHost, cfg.OllamaModel), cfg.OllamaModel, nil
	}
}

func loadFixture(dir string) (benchmark.GroundTruth, string, string, error) {
	gtPath := filepath.Join(dir, "ground_truth.json")
	gtData, err := os.ReadFile(gtPath)
	if err != nil {
		return benchmark.GroundTruth{}, "", "", fmt.Errorf("missing ground_truth.json: %w", err)
	}

	var gt benchmark.GroundTruth
	if err := json.Unmarshal(gtData, &gt); err != nil {
		return benchmark.GroundTruth{}, "", "", fmt.Errorf("invalid ground_truth.json: %w", err)
	}

	// Find the image file.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return benchmark.GroundTruth{}, "", "", err
	}
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if mimeType, ok := imageExts[ext]; ok {
			return gt, filepath.Join(dir, e.Name()), mimeType, nil
		}
	}

	return benchmark.GroundTruth{}, "", "", fmt.Errorf("no image file found (expected .jpg/.jpeg/.png/.webp)")
}

func buildSummary(backend, model string, results []benchmark.MatchResult) Summary {
	var totalItem, totalQty float64
	for _, r := range results {
		totalItem += r.ItemAccuracy
		totalQty += r.QuantityAccuracy
	}
	n := float64(len(results))
	return Summary{
		Backend:             backend,
		Model:               model,
		Fixtures:            len(results),
		Results:             results,
		AvgItemAccuracy:     totalItem / n,
		AvgQuantityAccuracy: totalQty / n,
	}
}

func printSummary(s Summary) {
	fmt.Printf("Backend: %s  Model: %s\n", s.Backend, s.Model)
	fmt.Printf("Fixtures: %d\n\n", s.Fixtures)

	for _, r := range s.Results {
		fmt.Printf("── %s ──\n", r.Fixture)
		fmt.Printf("  Expected: %d  Detected: %d  Matched: %d  Qty correct: %d\n",
			r.Expected, r.Detected, r.ItemMatches, r.QuantityMatches)
		fmt.Printf("  Item accuracy:     %.0f%%\n", r.ItemAccuracy*100)
		fmt.Printf("  Quantity accuracy: %.0f%%\n", r.QuantityAccuracy*100)
		fmt.Println()

		for _, ir := range r.Items {
			if ir.Detected == nil {
				fmt.Printf("  ✗ MISS   expected: %s (qty %d)\n",
					ir.Expected.Name, ir.Expected.Quantity)
			} else if !ir.QuantityMatch {
				fmt.Printf("  ~ MATCH  expected: %s (qty %d)  →  got: %s (qty %s)\n",
					ir.Expected.Name, ir.Expected.Quantity,
					ir.Detected.Name, ir.Detected.Quantity)
			} else {
				fmt.Printf("  ✓ MATCH  expected: %s (qty %d)  →  got: %s (qty %s)\n",
					ir.Expected.Name, ir.Expected.Quantity,
					ir.Detected.Name, ir.Detected.Quantity)
			}
		}

		if len(r.Extra) > 0 {
			fmt.Println()
			for _, e := range r.Extra {
				fmt.Printf("  + EXTRA  %s (qty %s)\n", e.Name, e.Quantity)
			}
		}
		fmt.Println()
	}

	fmt.Printf("━━━ OVERALL ━━━\n")
	fmt.Printf("  Avg item accuracy:     %.0f%%\n", s.AvgItemAccuracy*100)
	fmt.Printf("  Avg quantity accuracy: %.0f%%\n", s.AvgQuantityAccuracy*100)
}

