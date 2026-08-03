package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New returns an slog.Logger configured for the given application environment.
func New(appEnv string) *slog.Logger {
	env := strings.ToLower(strings.TrimSpace(appEnv))
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if env == "development" || env == "dev" || env == "local" {
		opts.Level = slog.LevelDebug
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
