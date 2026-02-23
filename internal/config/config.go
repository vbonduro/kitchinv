package config

import "os"

type Config struct {
	ListenAddr    string
	DBPath        string
	VisionBackend string
	OllamaHost    string
	OllamaModel   string
	ClaudeAPIKey  string
	ClaudeModel   string
	PhotoBackend  string
	PhotoPath     string
	LogLevel      string
	LogFile       string
	TestMode      bool
}

func Load() *Config {
	return &Config{
		ListenAddr:    getEnv("LISTEN_ADDR", ":8080"),
		DBPath:        getEnv("DB_PATH", "/data/kitchinv.db"),
		VisionBackend: getEnv("VISION_BACKEND", "ollama"),
		OllamaHost:    getEnv("OLLAMA_HOST", "http://localhost:11434"),
		OllamaModel:   getEnv("OLLAMA_MODEL", "moondream"),
		ClaudeAPIKey:  getEnv("CLAUDE_API_KEY", ""),
		ClaudeModel:   getEnv("CLAUDE_MODEL", "claude-opus-4-6"),
		PhotoBackend:  getEnv("PHOTO_BACKEND", "local"),
		PhotoPath:     getEnv("PHOTO_LOCAL_PATH", "/data/photos"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		LogFile:       getEnv("LOG_FILE", ""),
		TestMode:      os.Getenv("KITCHINV_TEST_MODE") == "1",
	}
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}
