package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New creates and configures a structured JSON logger based on the provided log level string.
func New(levelStr string) *slog.Logger {
	var level slog.Level

	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	// We use JSONHandler to emit structured JSON logs suitable for Grafana Loki / CloudWatch.
	handler := slog.NewJSONHandler(os.Stdout, opts)

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}
