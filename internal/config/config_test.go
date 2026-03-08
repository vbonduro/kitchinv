package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	cfg := Load()

	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.ListenAddr)
	assert.NotEmpty(t, cfg.DBPath)
	assert.NotEmpty(t, cfg.VisionBackend)
}

func TestLoadKeyFromFile(t *testing.T) {
	dir := t.TempDir()

	geminiKey := filepath.Join(dir, "gemini_key")
	claudeKey := filepath.Join(dir, "claude_key")
	require.NoError(t, os.WriteFile(geminiKey, []byte("gemini-key-from-file\n"), 0600))
	require.NoError(t, os.WriteFile(claudeKey, []byte("claude-key-from-file"), 0600))

	t.Setenv("GEMINI_API_KEY_FILE", geminiKey)
	t.Setenv("CLAUDE_API_KEY_FILE", claudeKey)

	cfg := Load()

	// Keys read from file, trailing newline stripped
	assert.Equal(t, "gemini-key-from-file", cfg.GeminiAPIKey)
	assert.Equal(t, "claude-key-from-file", cfg.ClaudeAPIKey)
}

func TestLoadKeyFileOverridesEnvVar(t *testing.T) {
	dir := t.TempDir()
	geminiKey := filepath.Join(dir, "gemini_key")
	require.NoError(t, os.WriteFile(geminiKey, []byte("key-from-file"), 0600))

	t.Setenv("GEMINI_API_KEY", "key-from-env")
	t.Setenv("GEMINI_API_KEY_FILE", geminiKey)

	cfg := Load()

	assert.Equal(t, "key-from-file", cfg.GeminiAPIKey)
}

func TestLoadKeyFileNotFound(t *testing.T) {
	t.Setenv("GEMINI_API_KEY_FILE", "/nonexistent/path/gemini_key")

	cfg := Load()

	// Falls back to empty string rather than panicking
	assert.Empty(t, cfg.GeminiAPIKey)
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
