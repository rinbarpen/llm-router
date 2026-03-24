package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime settings for the Go backend.
type Config struct {
	Host              string
	Port              int
	PostgresDSN       string
	MigrateFromSQLite bool
	SQLiteMainPath    string
	SQLiteMonitorPath string
	RequireAuth       bool
	AllowLocalNoAuth  bool
}

func Load() (Config, error) {
	cfg := Config{
		Host:              getenvDefault("LLM_ROUTER_HOST", "0.0.0.0"),
		Port:              8000,
		PostgresDSN:       resolvePostgresDSN(),
		MigrateFromSQLite: parseBoolDefault("LLM_ROUTER_MIGRATE_FROM_SQLITE", true),
		SQLiteMainPath:    resolveSQLitePath("LLM_ROUTER_SQLITE_MAIN_PATH", "LLM_ROUTER_DATABASE_URL", "data/llm_router.db"),
		SQLiteMonitorPath: resolveSQLitePath("LLM_ROUTER_SQLITE_MONITOR_PATH", "LLM_ROUTER_MONITOR_DATABASE_URL", "data/llm_datas.db"),
		RequireAuth:       parseBoolDefault("LLM_ROUTER_REQUIRE_AUTH", false),
		AllowLocalNoAuth:  parseBoolDefault("LLM_ROUTER_ALLOW_LOCAL_WITHOUT_AUTH", true),
	}

	if portRaw := strings.TrimSpace(os.Getenv("LLM_ROUTER_PORT")); portRaw != "" {
		p, err := strconv.Atoi(portRaw)
		if err != nil || p < 1 || p > 65535 {
			return Config{}, fmt.Errorf("invalid LLM_ROUTER_PORT=%q", portRaw)
		}
		cfg.Port = p
	}

	if strings.TrimSpace(cfg.PostgresDSN) == "" {
		return Config{}, errors.New("postgres DSN is required (set LLM_ROUTER_PG_DSN or LLM_ROUTER_POSTGRES_DSN)")
	}

	return cfg, nil
}

func resolvePostgresDSN() string {
	if v := strings.TrimSpace(os.Getenv("LLM_ROUTER_PG_DSN")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("LLM_ROUTER_POSTGRES_DSN")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("LLM_ROUTER_DATABASE_URL")); strings.HasPrefix(v, "postgres") {
		return v
	}
	return "postgres://localhost:5432/llm_router?sslmode=disable"
}

func resolveSQLitePath(pathEnv string, urlEnv string, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(pathEnv)); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(urlEnv)); v != "" {
		if p := normalizeSQLitePath(v); p != "" {
			return p
		}
	}
	return fallback
}

func normalizeSQLitePath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "sqlite+aiosqlite:///") {
		return strings.TrimPrefix(trimmed, "sqlite+aiosqlite:///")
	}
	if strings.HasPrefix(trimmed, "sqlite:///") {
		return strings.TrimPrefix(trimmed, "sqlite:///")
	}
	if strings.HasPrefix(trimmed, "sqlite://") {
		return strings.TrimPrefix(trimmed, "sqlite://")
	}
	return trimmed
}

func parseBoolDefault(key string, defaultVal bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return defaultVal
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultVal
	}
}

func getenvDefault(key string, defaultVal string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return defaultVal
}
