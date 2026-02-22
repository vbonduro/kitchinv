package logging

import (
	"io"
	"log/slog"
	"os"
)

// New creates a *slog.Logger writing JSON to stderr and optionally to logFile.
// The returned cleanup func closes the log file if one was opened; callers must
// defer it. Callers that want package-level slog calls to use this logger should
// call slog.SetDefault(logger) after construction.
func New(level, logFile string) (*slog.Logger, func(), error) {
	lvl := parseLevel(level)

	writers := []io.Writer{os.Stderr}
	cleanup := func() {}

	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return nil, nil, err
		}
		writers = append(writers, f)
		cleanup = func() { _ = f.Close() }
	}

	w := io.MultiWriter(writers...)
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl})
	logger := slog.New(handler)
	return logger, cleanup, nil
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
