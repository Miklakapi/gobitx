package logger

import (
	"io"
	"log/slog"
	"os"
)

type colorWriter struct {
	writer io.Writer
}

func SetupLogger(verbose bool) {
	level := slog.LevelWarn

	if verbose {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(&colorWriter{writer: os.Stderr}, &slog.HandlerOptions{
		Level: level,
	})

	slog.SetDefault(slog.New(handler))
}

func (w *colorWriter) Write(p []byte) (int, error) {
	_, err := w.writer.Write([]byte("\033[36m"))
	if err != nil {
		return 0, err
	}

	n, err := w.writer.Write(p)

	_, resetErr := w.writer.Write([]byte("\033[0m"))
	if err != nil {
		return n, err
	}

	return n, resetErr
}
