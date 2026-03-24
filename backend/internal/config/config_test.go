package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("LLM_ROUTER_PG_DSN", "")
	t.Setenv("LLM_ROUTER_DATABASE_URL", "")
	t.Setenv("LLM_ROUTER_MONITOR_DATABASE_URL", "")
	t.Setenv("LLM_ROUTER_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8000 {
		t.Fatalf("Port = %d, want 8000", cfg.Port)
	}
	if cfg.PostgresDSN == "" {
		t.Fatalf("PostgresDSN should have default value")
	}
	if cfg.SQLiteMainPath != "data/llm_router.db" {
		t.Fatalf("SQLiteMainPath = %q", cfg.SQLiteMainPath)
	}
	if cfg.SQLiteMonitorPath != "data/llm_datas.db" {
		t.Fatalf("SQLiteMonitorPath = %q", cfg.SQLiteMonitorPath)
	}
	if cfg.RequireAuth {
		t.Fatalf("RequireAuth should default to false")
	}
	if !cfg.AllowLocalNoAuth {
		t.Fatalf("AllowLocalNoAuth should default to true")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("LLM_ROUTER_PG_DSN", "postgres://user:pass@localhost:5432/db")
	t.Setenv("LLM_ROUTER_PORT", "19000")
	t.Setenv("LLM_ROUTER_MIGRATE_FROM_SQLITE", "false")
	t.Setenv("LLM_ROUTER_SQLITE_MAIN_PATH", "/tmp/main.db")
	t.Setenv("LLM_ROUTER_SQLITE_MONITOR_PATH", "/tmp/monitor.db")
	t.Setenv("LLM_ROUTER_REQUIRE_AUTH", "true")
	t.Setenv("LLM_ROUTER_ALLOW_LOCAL_WITHOUT_AUTH", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 19000 {
		t.Fatalf("Port = %d, want 19000", cfg.Port)
	}
	if cfg.PostgresDSN != "postgres://user:pass@localhost:5432/db" {
		t.Fatalf("unexpected dsn: %s", cfg.PostgresDSN)
	}
	if cfg.MigrateFromSQLite {
		t.Fatalf("MigrateFromSQLite should be false")
	}
	if cfg.SQLiteMainPath != "/tmp/main.db" || cfg.SQLiteMonitorPath != "/tmp/monitor.db" {
		t.Fatalf("unexpected sqlite paths: %q %q", cfg.SQLiteMainPath, cfg.SQLiteMonitorPath)
	}
	if !cfg.RequireAuth {
		t.Fatalf("RequireAuth should be true from env")
	}
	if cfg.AllowLocalNoAuth {
		t.Fatalf("AllowLocalNoAuth should be false from env")
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("LLM_ROUTER_PORT", "abc")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
