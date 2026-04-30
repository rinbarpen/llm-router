package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLoggerWritesToBufferAndFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "llm-router.log")
	var stdout bytes.Buffer

	logger, closeFn, err := NewLogger(Options{
		Level:         "info",
		Format:        "text",
		StdoutEnabled: true,
		Stdout:        &stdout,
		FilePath:      path,
	})
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer closeFn()

	logger.Info("logger ready", slog.String("component", "test"))

	fileRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	fileText := string(fileRaw)
	if !strings.Contains(fileText, "logger ready") {
		t.Fatalf("file log missing message: %s", fileText)
	}
	if !strings.Contains(stdout.String(), "logger ready") {
		t.Fatalf("stdout log missing message: %s", stdout.String())
	}
}
