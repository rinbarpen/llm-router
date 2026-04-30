package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds runtime settings for the Go backend.
type Config struct {
	Host              string
	Port              int
	SQLitePath        string
	MigrateFromSQLite bool
	SQLiteMainPath    string
	SQLiteMonitorPath string
	ModelConfigPath   string
	RequireAuth       bool
	AllowLocalNoAuth  bool
	Logging           LoggingConfig
}

func Load() (Config, error) {
	_ = loadDotEnvIfPresent()

	cfg := Config{
		Host:              getenvDefault("LLM_ROUTER_HOST", "0.0.0.0"),
		Port:              18000,
		SQLitePath:        resolveRuntimeSQLitePath(),
		MigrateFromSQLite: parseBoolDefault("LLM_ROUTER_MIGRATE_FROM_SQLITE", true),
		SQLiteMainPath:    resolveSQLitePath("LLM_ROUTER_SQLITE_MAIN_PATH", "LLM_ROUTER_DATABASE_URL", "data/llm_router.db"),
		SQLiteMonitorPath: resolveSQLitePath("LLM_ROUTER_SQLITE_MONITOR_PATH", "LLM_ROUTER_MONITOR_DATABASE_URL", "data/llm_datas.db"),
		ModelConfigPath:   resolveModelConfigPathEnv(),
		RequireAuth:       parseBoolDefault("LLM_ROUTER_REQUIRE_AUTH", false),
		AllowLocalNoAuth:  parseBoolDefault("LLM_ROUTER_ALLOW_LOCAL_WITHOUT_AUTH", true),
		Logging: LoggingConfig{
			Level:         "info",
			Format:        "text",
			StdoutEnabled: true,
		},
	}

	if modelCfg, err := loadRuntimeModelConfig(cfg.ModelConfigPath); err == nil {
		applyModelConfig(&cfg, modelCfg)
	}

	if portRaw := strings.TrimSpace(os.Getenv("LLM_ROUTER_PORT")); portRaw != "" {
		p, err := strconv.Atoi(portRaw)
		if err != nil || p < 1 || p > 65535 {
			return Config{}, fmt.Errorf("invalid LLM_ROUTER_PORT=%q", portRaw)
		}
		cfg.Port = p
	}
	if level := strings.TrimSpace(os.Getenv("LLM_ROUTER_LOG_LEVEL")); level != "" {
		cfg.Logging.Level = strings.ToLower(level)
	}
	if format := strings.TrimSpace(os.Getenv("LLM_ROUTER_LOG_FORMAT")); format != "" {
		cfg.Logging.Format = strings.ToLower(format)
	}
	if stdoutRaw := strings.TrimSpace(os.Getenv("LLM_ROUTER_LOG_STDOUT")); stdoutRaw != "" {
		cfg.Logging.StdoutEnabled = parseBoolDefault("LLM_ROUTER_LOG_STDOUT", cfg.Logging.StdoutEnabled)
	}
	if filePath := strings.TrimSpace(os.Getenv("LLM_ROUTER_LOG_FILE_PATH")); filePath != "" {
		cfg.Logging.FilePath = filePath
	}

	if strings.TrimSpace(cfg.SQLitePath) == "" {
		return Config{}, errors.New("sqlite path is required (set LLM_ROUTER_SQLITE_PATH or LLM_ROUTER_DATABASE_URL)")
	}
	cfg.Logging = normalizeLoggingConfig(cfg.Logging)

	return cfg, nil
}

func loadRuntimeModelConfig(path string) (RouterModelConfig, error) {
	resolved, err := ResolveModelConfigPath(path)
	if err != nil {
		return RouterModelConfig{}, err
	}
	return LoadRouterModelConfigFromTOML(resolved)
}

func applyModelConfig(cfg *Config, modelCfg RouterModelConfig) {
	if cfg == nil {
		return
	}
	if modelCfg.Logging != nil {
		if strings.TrimSpace(modelCfg.Logging.Level) != "" {
			cfg.Logging.Level = modelCfg.Logging.Level
		}
		if strings.TrimSpace(modelCfg.Logging.Format) != "" {
			cfg.Logging.Format = modelCfg.Logging.Format
		}
		cfg.Logging.StdoutEnabled = modelCfg.Logging.StdoutEnabled
		if strings.TrimSpace(modelCfg.Logging.FilePath) != "" {
			cfg.Logging.FilePath = modelCfg.Logging.FilePath
		}
	}
}

func normalizeLoggingConfig(cfg LoggingConfig) LoggingConfig {
	cfg.Level = strings.ToLower(strings.TrimSpace(cfg.Level))
	switch cfg.Level {
	case "debug", "info", "warn", "error":
	default:
		cfg.Level = "info"
	}
	cfg.Format = strings.ToLower(strings.TrimSpace(cfg.Format))
	switch cfg.Format {
	case "text", "json":
	default:
		cfg.Format = "text"
	}
	cfg.FilePath = strings.TrimSpace(cfg.FilePath)
	return cfg
}

func resolveRuntimeSQLitePath() string {
	if v := strings.TrimSpace(os.Getenv("LLM_ROUTER_SQLITE_PATH")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("LLM_ROUTER_DATABASE_URL")); v != "" {
		if p := normalizeSQLitePath(v); p != "" && !strings.HasPrefix(strings.ToLower(p), "postgres") {
			return p
		}
	}
	return "data/llm_router.db"
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

// resolveModelConfigPathEnv matches Python: LLM_ROUTER_MODEL_CONFIG overrides LLM_ROUTER_MODEL_CONFIG_FILE.
func resolveModelConfigPathEnv() string {
	if v := strings.TrimSpace(os.Getenv("LLM_ROUTER_MODEL_CONFIG")); v != "" {
		return v
	}
	return getenvDefault("LLM_ROUTER_MODEL_CONFIG_FILE", "router.toml")
}

func loadDotEnvIfPresent() error {
	if parseBoolDefault("LLM_ROUTER_DISABLE_DOTENV", false) {
		return nil
	}
	path := filepath.Join(".", ".env")
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		} else {
			value = strings.Trim(value, `"'`)
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}
