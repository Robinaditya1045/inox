package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type contextKey string

const LoggerKey contextKey = "logger"

// FromContext extracts the logger from the context. If none exists, it returns the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(LoggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// WithLogger returns a new context with the provided logger.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey, logger)
}

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
