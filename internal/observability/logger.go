package observability

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

type LoggerConfig struct {
	Level  string
	Format string
}

func NewLogger(cfg LoggerConfig) *slog.Logger {
	return buildLogger(cfg, os.Stdout)
}

// Сейчас оба пишут в stdout, но разделение позволяет
// в будущем направить access log в отдельный поток.
func NewAccessLogger(cfg LoggerConfig) *slog.Logger {
	return buildLogger(cfg, os.Stdout).With("logger", "access")
}

func buildLogger(cfg LoggerConfig, w io.Writer) *slog.Logger {
	level := parseLogLevel(cfg.Level)
	format := strings.ToLower(strings.TrimSpace(cfg.Format))

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	default:
		handler = slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	}
	return slog.New(handler)
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
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
