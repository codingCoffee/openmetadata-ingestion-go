// Package logging configures the process-wide slog logger from the workflow's
// loggerLevel, with an optional CLI override.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Setup installs a text slog handler at the level derived from level (one of
// DEBUG/INFO/WARN/ERROR, case-insensitive). Unknown or empty values default to INFO.
// It returns the configured logger, which is also set as the default.
func Setup(level string) *slog.Logger {
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: parse(level)})
	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}

func parse(level string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
