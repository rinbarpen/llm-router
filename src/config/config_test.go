package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("LLM_ROUTER_DISABLE_DOTENV", "true")
	t.Setenv("LLM_ROUTER_SQLITE_PATH", "")
	t.Setenv("LLM_ROUTER_DATABASE_URL", "")
	t.Setenv("LLM_ROUTER_MONITOR_DATABASE_URL", "")
	t.Setenv("LLM_ROUTER_PORT", "")
	t.Setenv("LLM_ROUTER_LOG_LEVEL", "")
	t.Setenv("LLM_ROUTER_LOG_FORMAT", "")
	t.Setenv("LLM_ROUTER_LOG_STDOUT", "")
	t.Setenv("LLM_ROUTER_LOG_FILE_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 18000 {
		t.Fatalf("Port = %d, want 18000", cfg.Port)
	}
	if cfg.SQLitePath != "data/llm_router.db" {
		t.Fatalf("SQLitePath = %q, want data/llm_router.db", cfg.SQLitePath)
	}
	if cfg.SQLiteMainPath != "data/llm_router.db" {
		t.Fatalf("SQLiteMainPath = %q", cfg.SQLiteMainPath)
	}
	if cfg.SQLiteMonitorPath != "data/llm_datas.db" {
		t.Fatalf("SQLiteMonitorPath = %q", cfg.SQLiteMonitorPath)
	}
	if cfg.ModelConfigPath != "router.toml" {
		t.Fatalf("ModelConfigPath = %q, want router.toml", cfg.ModelConfigPath)
	}
	if cfg.RequireAuth {
		t.Fatalf("RequireAuth should default to false")
	}
	if !cfg.AllowLocalNoAuth {
		t.Fatalf("AllowLocalNoAuth should default to true")
	}
	if cfg.Logging.Level != "info" {
		t.Fatalf("Logging.Level = %q, want info", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Fatalf("Logging.Format = %q, want text", cfg.Logging.Format)
	}
	if !cfg.Logging.StdoutEnabled {
		t.Fatalf("Logging.StdoutEnabled should default to true")
	}
	if cfg.Logging.FilePath != "" {
		t.Fatalf("Logging.FilePath = %q, want empty", cfg.Logging.FilePath)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("LLM_ROUTER_DISABLE_DOTENV", "true")
	t.Setenv("LLM_ROUTER_SQLITE_PATH", "/tmp/runtime.db")
	t.Setenv("LLM_ROUTER_PORT", "19000")
	t.Setenv("LLM_ROUTER_MIGRATE_FROM_SQLITE", "false")
	t.Setenv("LLM_ROUTER_SQLITE_MAIN_PATH", "/tmp/main.db")
	t.Setenv("LLM_ROUTER_SQLITE_MONITOR_PATH", "/tmp/monitor.db")
	t.Setenv("LLM_ROUTER_MODEL_CONFIG_FILE", "/tmp/router.toml")
	t.Setenv("LLM_ROUTER_REQUIRE_AUTH", "true")
	t.Setenv("LLM_ROUTER_ALLOW_LOCAL_WITHOUT_AUTH", "false")
	t.Setenv("LLM_ROUTER_LOG_LEVEL", "debug")
	t.Setenv("LLM_ROUTER_LOG_FORMAT", "json")
	t.Setenv("LLM_ROUTER_LOG_STDOUT", "false")
	t.Setenv("LLM_ROUTER_LOG_FILE_PATH", "/tmp/llm-router.log")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 19000 {
		t.Fatalf("Port = %d, want 19000", cfg.Port)
	}
	if cfg.SQLitePath != "/tmp/runtime.db" {
		t.Fatalf("unexpected sqlite path: %s", cfg.SQLitePath)
	}
	if cfg.MigrateFromSQLite {
		t.Fatalf("MigrateFromSQLite should be false")
	}
	if cfg.SQLiteMainPath != "/tmp/main.db" || cfg.SQLiteMonitorPath != "/tmp/monitor.db" {
		t.Fatalf("unexpected sqlite paths: %q %q", cfg.SQLiteMainPath, cfg.SQLiteMonitorPath)
	}
	if cfg.ModelConfigPath != "/tmp/router.toml" {
		t.Fatalf("unexpected model config path: %q", cfg.ModelConfigPath)
	}
	if !cfg.RequireAuth {
		t.Fatalf("RequireAuth should be true from env")
	}
	if cfg.AllowLocalNoAuth {
		t.Fatalf("AllowLocalNoAuth should be false from env")
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("Logging.Level = %q, want debug", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("Logging.Format = %q, want json", cfg.Logging.Format)
	}
	if cfg.Logging.StdoutEnabled {
		t.Fatalf("Logging.StdoutEnabled should be false from env")
	}
	if cfg.Logging.FilePath != "/tmp/llm-router.log" {
		t.Fatalf("Logging.FilePath = %q, want /tmp/llm-router.log", cfg.Logging.FilePath)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("LLM_ROUTER_DISABLE_DOTENV", "true")
	t.Setenv("LLM_ROUTER_PORT", "abc")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestLoadReadsDotEnvWithoutOverridingExistingEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(strings.Join([]string{
		"LLM_ROUTER_SQLITE_PATH=/tmp/dotenv-router.db",
		"LLM_ROUTER_PORT=19001",
		"LLM_ROUTER_MODEL_CONFIG_FILE=dotenv-router.toml",
		"VAPI_API_KEY=dotenv-key",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q): %v", dir, err)
	}

	t.Setenv("LLM_ROUTER_DISABLE_DOTENV", "false")
	t.Setenv("LLM_ROUTER_SQLITE_PATH", "")
	t.Setenv("LLM_ROUTER_PORT", "")
	t.Setenv("LLM_ROUTER_MODEL_CONFIG_FILE", "")
	t.Setenv("VAPI_API_KEY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 19001 {
		t.Fatalf("Port = %d, want 19001", cfg.Port)
	}
	if cfg.SQLitePath != "/tmp/dotenv-router.db" {
		t.Fatalf("SQLitePath = %q", cfg.SQLitePath)
	}
	if cfg.ModelConfigPath != "dotenv-router.toml" {
		t.Fatalf("ModelConfigPath = %q", cfg.ModelConfigPath)
	}
	if got := os.Getenv("VAPI_API_KEY"); got != "dotenv-key" {
		t.Fatalf("VAPI_API_KEY = %q", got)
	}

	t.Setenv("LLM_ROUTER_PORT", "19002")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() with env override error = %v", err)
	}
	if cfg.Port != 19002 {
		t.Fatalf("Port with env override = %d, want 19002", cfg.Port)
	}
}

func TestLoadReadsLoggingFromRouterTOML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "router.toml"), []byte(strings.Join([]string{
		"[logging]",
		`level = "warn"`,
		`format = "json"`,
		"stdout_enabled = false",
		`file_path = "/tmp/router.log"`,
	}, "\n")), 0o644); err != nil {
		t.Fatalf("write router.toml: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q): %v", dir, err)
	}

	t.Setenv("LLM_ROUTER_DISABLE_DOTENV", "true")
	t.Setenv("LLM_ROUTER_SQLITE_PATH", "")
	t.Setenv("LLM_ROUTER_DATABASE_URL", "")
	t.Setenv("LLM_ROUTER_LOG_LEVEL", "")
	t.Setenv("LLM_ROUTER_LOG_FORMAT", "")
	t.Setenv("LLM_ROUTER_LOG_STDOUT", "")
	t.Setenv("LLM_ROUTER_LOG_FILE_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("Logging.Level = %q, want warn", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("Logging.Format = %q, want json", cfg.Logging.Format)
	}
	if cfg.Logging.StdoutEnabled {
		t.Fatalf("Logging.StdoutEnabled should be false from router.toml")
	}
	if cfg.Logging.FilePath != "/tmp/router.log" {
		t.Fatalf("Logging.FilePath = %q, want /tmp/router.log", cfg.Logging.FilePath)
	}
}

func TestLoadEnvOverridesRouterTOMLLogging(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "router.toml"), []byte(strings.Join([]string{
		"[logging]",
		`level = "warn"`,
		`format = "json"`,
		"stdout_enabled = false",
		`file_path = "/tmp/router.log"`,
	}, "\n")), 0o644); err != nil {
		t.Fatalf("write router.toml: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q): %v", dir, err)
	}

	t.Setenv("LLM_ROUTER_DISABLE_DOTENV", "true")
	t.Setenv("LLM_ROUTER_SQLITE_PATH", "")
	t.Setenv("LLM_ROUTER_DATABASE_URL", "")
	t.Setenv("LLM_ROUTER_LOG_LEVEL", "error")
	t.Setenv("LLM_ROUTER_LOG_FORMAT", "text")
	t.Setenv("LLM_ROUTER_LOG_STDOUT", "true")
	t.Setenv("LLM_ROUTER_LOG_FILE_PATH", "/tmp/env.log")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Logging.Level != "error" {
		t.Fatalf("Logging.Level = %q, want error", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Fatalf("Logging.Format = %q, want text", cfg.Logging.Format)
	}
	if !cfg.Logging.StdoutEnabled {
		t.Fatalf("Logging.StdoutEnabled should be true from env")
	}
	if cfg.Logging.FilePath != "/tmp/env.log" {
		t.Fatalf("Logging.FilePath = %q, want /tmp/env.log", cfg.Logging.FilePath)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
