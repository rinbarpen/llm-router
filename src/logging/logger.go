package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type Options struct {
	Level         string
	Format        string
	StdoutEnabled bool
	Stdout        io.Writer
	FilePath      string
}

func NewLogger(opts Options) (*slog.Logger, func() error, error) {
	level := parseLevel(opts.Level)
	writer, closeFn, err := buildWriter(opts)
	if err != nil {
		return nil, nil, err
	}
	handlerOptions := &slog.HandlerOptions{Level: level}
	format := strings.ToLower(strings.TrimSpace(opts.Format))
	var handler slog.Handler
	switch format {
	case "", "text":
		handler = slog.NewTextHandler(writer, handlerOptions)
	case "json":
		handler = slog.NewJSONHandler(writer, handlerOptions)
	default:
		_ = closeFn()
		return nil, nil, fmt.Errorf("unsupported log format %q", opts.Format)
	}
	return slog.New(handler), closeFn, nil
}

func buildWriter(opts Options) (io.Writer, func() error, error) {
	writers := make([]io.Writer, 0, 2)
	if opts.StdoutEnabled {
		if opts.Stdout != nil {
			writers = append(writers, opts.Stdout)
		} else {
			writers = append(writers, os.Stdout)
		}
	}

	var file *os.File
	if strings.TrimSpace(opts.FilePath) != "" {
		var err error
		file, err = os.OpenFile(strings.TrimSpace(opts.FilePath), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file: %w", err)
		}
		writers = append(writers, file)
	}
	if len(writers) == 0 {
		writers = append(writers, io.Discard)
	}
	closeFn := func() error {
		if file != nil {
			return file.Close()
		}
		return nil
	}
	return io.MultiWriter(writers...), closeFn, nil
}

func parseLevel(raw string) slog.Leveler {
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
