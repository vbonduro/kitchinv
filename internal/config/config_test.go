package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	cfg := Load()

	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.ListenAddr)
	assert.NotEmpty(t, cfg.DBPath)
	assert.NotEmpty(t, cfg.VisionBackend)
}

func TestLoadCustomValues(t *testing.T) {
	t.Setenv("LISTEN_ADDR", ":9000")
	t.Setenv("DB_PATH", "/custom/db.sqlite")
	t.Setenv("VISION_BACKEND", "claude")
	t.Setenv("CLAUDE_API_KEY", "sk-test123")

	cfg := Load()

	assert.Equal(t, ":9000", cfg.ListenAddr)
	assert.Equal(t, "/custom/db.sqlite", cfg.DBPath)
	assert.Equal(t, "claude", cfg.VisionBackend)
	assert.Equal(t, "sk-test123", cfg.ClaudeAPIKey)
}
