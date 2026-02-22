package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/vbonduro/kitchinv/internal/config"
	"github.com/vbonduro/kitchinv/internal/db"
	"github.com/vbonduro/kitchinv/internal/logging"
	"github.com/vbonduro/kitchinv/internal/photostore/local"
	"github.com/vbonduro/kitchinv/internal/service"
	"github.com/vbonduro/kitchinv/internal/store"
	"github.com/vbonduro/kitchinv/internal/vision"
	claudevision "github.com/vbonduro/kitchinv/internal/vision/claude"
	ollamavision "github.com/vbonduro/kitchinv/internal/vision/ollama"
	"github.com/vbonduro/kitchinv/internal/web"
	"github.com/vbonduro/kitchinv/internal/web/templates"
)

func main() {
	cfg := config.Load()

	logger, cleanup, err := logging.New(cfg.LogLevel, cfg.LogFile)
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer cleanup()
	slog.SetDefault(logger)

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		return
	}
	defer func() {
		if err := database.Close(); err != nil {
			logger.Error("failed to close database", "error", err)
		}
	}()

	areaStore := store.NewAreaStore(database)
	photoStore := store.NewPhotoStore(database)
	itemStore := store.NewItemStore(database)

	visionAnalyzer, err := newVisionAnalyzer(cfg, logger)
	if err != nil {
		logger.Error("vision backend misconfigured", "error", err)
		os.Exit(1)
	}

	photoStg, err := local.NewLocalPhotoStore(cfg.PhotoPath)
	if err != nil {
		logger.Error("failed to initialize photo store", "error", err)
		return
	}

	areaService := service.NewAreaService(areaStore, photoStore, itemStore, visionAnalyzer, photoStg, logger)
	server := web.NewServer(areaService, templates.FS, photoStg, logger)

	if err := server.ListenAndServe(cfg.ListenAddr); err != nil {
		logger.Error("server error", "error", err)
	}
}

func newVisionAnalyzer(cfg *config.Config, logger *slog.Logger) (vision.VisionAnalyzer, error) {
	switch cfg.VisionBackend {
	case "claude":
		if cfg.ClaudeAPIKey == "" {
			return nil, fmt.Errorf("CLAUDE_API_KEY must be set when VISION_BACKEND=claude")
		}
		logger.Info("using Claude vision backend")
		return claudevision.NewClaudeAnalyzer(cfg.ClaudeAPIKey, cfg.ClaudeModel), nil
	default:
		logger.Info("using Ollama vision backend", "model", cfg.OllamaModel)
		return ollamavision.NewOllamaAnalyzer(cfg.OllamaHost, cfg.OllamaModel), nil
	}
}
