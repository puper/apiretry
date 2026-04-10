package observe

import (
	"log/slog"
	"os"

	"github.com/puper/apiretry/internal/config"
)

func NewLogger(cfg *config.LoggingConfig) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	if cfg.JSON {
		handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
		return slog.New(handler)
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
