package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Setup initializes the global structured logger.
func Setup(levelStr string) *slog.Logger {
	var level slog.Level

	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	log := slog.New(handler)
	slog.SetDefault(log)

	return log
}