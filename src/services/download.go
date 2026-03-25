package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadService handles simple artifact downloads used by local providers.
type DownloadService struct {
	client *http.Client
}

func NewDownloadService(timeout time.Duration) *DownloadService {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &DownloadService{client: &http.Client{Timeout: timeout}}
}

func (s *DownloadService) DownloadToFile(ctx context.Context, uri string, targetPath string) error {
	uri = strings.TrimSpace(uri)
	targetPath = strings.TrimSpace(targetPath)
	if uri == "" || targetPath == "" {
		return fmt.Errorf("uri and targetPath are required")
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	tmpPath := targetPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("commit download file: %w", err)
	}
	return nil
}
