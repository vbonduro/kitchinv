package config

import (
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	ListenAddr    string
	DBPath        string
	VisionBackend string
	OllamaHost    string
	OllamaModel   string
	ClaudeAPIKey  string
	ClaudeModel   string
	GeminiAPIKey  string
	GeminiModel   string
	PhotoBackend  string
	PhotoPath     string
	LogLevel      string
	LogFile       string
}

func Load() *Config {
	return &Config{
		ListenAddr:    getEnv("LISTEN_ADDR", ":8080"),
		DBPath:        getEnv("DB_PATH", "/data/kitchinv.db"),
		VisionBackend: getEnv("VISION_BACKEND", "ollama"),
		OllamaHost:    getEnv("OLLAMA_HOST", "http://localhost:11434"),
		OllamaModel:   getEnv("OLLAMA_MODEL", "moondream"),
		ClaudeAPIKey:  getSecret("CLAUDE_API_KEY", "CLAUDE_API_KEY_FILE"),
		ClaudeModel:   getEnv("CLAUDE_MODEL", "claude-opus-4-6"),
		GeminiAPIKey:  getSecret("GEMINI_API_KEY", "GEMINI_API_KEY_FILE"),
		GeminiModel:   getEnv("GEMINI_MODEL", "gemini-2.5-flash"),
		PhotoBackend:  getEnv("PHOTO_BACKEND", "local"),
		PhotoPath:     getEnv("PHOTO_LOCAL_PATH", "/data/photos"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		LogFile:       getEnv("LOG_FILE", ""),
	}
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}

// getSecret reads a secret value from a file if the fileEnvKey env var is set,
// otherwise falls back to the plain envKey env var. File contents are trimmed
// of whitespace so keys stored with a trailing newline work correctly.
// If the file path is set but the file cannot be read, returns empty string.
func getSecret(envKey, fileEnvKey string) string {
	if path, exists := os.LookupEnv(fileEnvKey); exists && path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Error("failed to read secret file", "env", fileEnvKey, "path", path, "error", err)
			return ""
		}
		return strings.TrimSpace(string(data))
	}
	return getEnv(envKey, "")
}
