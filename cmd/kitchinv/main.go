package main

import (
	"log"

	"github.com/vbonduro/kitchinv/internal/config"
	"github.com/vbonduro/kitchinv/internal/db"
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

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	areaStore := store.NewAreaStore(database)
	photoStore := store.NewPhotoStore(database)
	itemStore := store.NewItemStore(database)

	visionAnalyzer := newVisionAnalyzer(cfg)

	photoStg, err := local.NewLocalPhotoStore(cfg.PhotoPath)
	if err != nil {
		log.Fatalf("failed to initialize photo store: %v", err)
	}

	areaService := service.NewAreaService(areaStore, photoStore, itemStore, visionAnalyzer, photoStg)
	server := web.NewServer(areaService, templates.FS, photoStg)

	if err := server.ListenAndServe(cfg.ListenAddr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func newVisionAnalyzer(cfg *config.Config) vision.VisionAnalyzer {
	switch cfg.VisionBackend {
	case "claude":
		if cfg.ClaudeAPIKey == "" {
			log.Fatal("CLAUDE_API_KEY is required when VISION_BACKEND=claude")
		}
		log.Println("using Claude vision backend")
		return claudevision.NewClaudeAnalyzer(cfg.ClaudeAPIKey, cfg.ClaudeModel)
	default:
		log.Printf("using Ollama vision backend (%s)", cfg.OllamaModel)
		return ollamavision.NewOllamaAnalyzer(cfg.OllamaHost, cfg.OllamaModel)
	}
}
