package observability

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// LoggerConfig описывает только то, что реально нужно логгеру.
type LoggerConfig struct {
	Level  string
	Format string
}

// NewLogger создает и настраивает общий logger приложения.
func NewLogger(cfg LoggerConfig) *slog.Logger {
	level := parseLogLevel(cfg.Level)
	format := strings.ToLower(strings.TrimSpace(cfg.Format))

	var handler slog.Handler

	switch format {
	case "json":
		handler = newJSONHandler(os.Stdout, level)
	default:
		handler = newTextHandler(os.Stdout, level)
	}

	return slog.New(handler)
}

// parseLogLevel переводит строку из конфига в slog.Level.
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

// newTextHandler создает текстовый handler для локальной разработки.
func newTextHandler(w io.Writer, level slog.Level) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
	})
}

// newJSONHandler создает JSON handler для контейнерной среды.
func newJSONHandler(w io.Writer, level slog.Level) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
	})
}
