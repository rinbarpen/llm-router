package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveModelConfigPath finds router.toml (or another relative path) by walking up from cwd.
func ResolveModelConfigPath(relativeOrAbs string) (string, error) {
	p := strings.TrimSpace(relativeOrAbs)
	if p == "" {
		p = "router.toml"
	}
	if filepath.IsAbs(p) {
		if _, err := os.Stat(p); err != nil {
			return "", err
		}
		return p, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, p)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", fmt.Errorf("model config file not found: %q (searched upward from %s)", p, wd)
}
