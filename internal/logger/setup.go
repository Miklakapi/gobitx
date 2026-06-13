package logger

import (
	"log/slog"
	"os"
)

func SetupLogger(verbose bool) {
	level := slog.LevelWarn

	if verbose {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	slog.SetDefault(slog.New(handler))
}
