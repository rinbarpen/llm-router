package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRouterModelConfigFromTOML_Providers(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "router.toml")
	content := `
[server]
port = 18000

[[providers]]
name = "openai"
type = "openai"
base_url = "https://api.openai.com/v1"
api_key_env = "OPENAI_API_KEY"
[providers.settings]
timeout = 300
skip_git_repo_check = true
args_template = ["exec", "--json", "-m", "{model}", "{prompt}"]

[[providers]]
name = "gemini"
type = "gemini"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp router.toml: %v", err)
	}

	cfg, err := LoadRouterModelConfigFromTOML(path)
	if err != nil {
		t.Fatalf("LoadRouterModelConfigFromTOML() error = %v", err)
	}
	if len(cfg.Providers) != 2 {
		t.Fatalf("provider count = %d, want 2", len(cfg.Providers))
	}
	p0 := cfg.Providers[0]
	if p0.Name != "openai" || p0.Type != "openai" {
		t.Fatalf("unexpected first provider: %+v", p0)
	}
	if p0.Settings == nil {
		t.Fatalf("settings should not be nil")
	}
	if timeout, ok := numberAsInt64(p0.Settings["timeout"]); !ok || timeout != 300 {
		t.Fatalf("timeout = %#v, want 300", p0.Settings["timeout"])
	}
	if p0.Settings["skip_git_repo_check"] != true {
		t.Fatalf("skip_git_repo_check = %#v, want true", p0.Settings["skip_git_repo_check"])
	}
	arr, ok := p0.Settings["args_template"].([]any)
	if !ok || len(arr) != 5 {
		t.Fatalf("args_template parse failed: %#v", p0.Settings["args_template"])
	}
	p1 := cfg.Providers[1]
	if p1.Name != "gemini" || p1.Type != "gemini" {
		t.Fatalf("unexpected second provider: %+v", p1)
	}
}

func TestLoadRouterModelConfigFromTOML_ProviderSettingsAPIPool(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "router.toml")
	content := `
[[providers]]
name = "vapi"
type = "openai"

[providers.settings]
api_base_urls = ["https://api.vveai.com", "https://api.gpt.ge", "https://api.v3.cm"]
latency_degrade_threshold_ms = 3000
cooldown_seconds = 30
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp router.toml: %v", err)
	}

	cfg, err := LoadRouterModelConfigFromTOML(path)
	if err != nil {
		t.Fatalf("LoadRouterModelConfigFromTOML() error = %v", err)
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("provider count = %d, want 1", len(cfg.Providers))
	}
	pool, ok := cfg.Providers[0].Settings["api_base_urls"].([]any)
	if !ok || len(pool) != 3 {
		t.Fatalf("api_base_urls parse failed: %#v", cfg.Providers[0].Settings["api_base_urls"])
	}
	if pool[0] != "https://api.vveai.com" || pool[2] != "https://api.v3.cm" {
		t.Fatalf("unexpected api_base_urls: %#v", pool)
	}
	if threshold, ok := numberAsInt64(cfg.Providers[0].Settings["latency_degrade_threshold_ms"]); !ok || threshold != 3000 {
		t.Fatalf("latency_degrade_threshold_ms = %#v, want 3000", cfg.Providers[0].Settings["latency_degrade_threshold_ms"])
	}
}

func numberAsInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	default:
		return 0, false
	}
}

func TestLoadRouterModelConfigFromTOML_RoutingPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "router.toml")
	content := `
[routing]
load_balance_strategy = "weighted"
channel_fallback = ["openrouter", "openai"]

[routing.provider_weights]
openrouter = 3
openai = 1

[routing.circuit_breaker]
enabled = true
failure_threshold = 5
cooldown_seconds = 40
half_open_max_requests = 2
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp router.toml: %v", err)
	}

	cfg, err := LoadRouterModelConfigFromTOML(path)
	if err != nil {
		t.Fatalf("LoadRouterModelConfigFromTOML() error = %v", err)
	}
	if cfg.Routing == nil {
		t.Fatalf("routing should not be nil")
	}
	if cfg.Routing.LoadBalanceStrategy != "weighted" {
		t.Fatalf("unexpected strategy: %s", cfg.Routing.LoadBalanceStrategy)
	}
	if len(cfg.Routing.ChannelFallback) != 2 {
		t.Fatalf("unexpected channel_fallback length: %d", len(cfg.Routing.ChannelFallback))
	}
	if cfg.Routing.ProviderWeights["openrouter"] != 3 {
		t.Fatalf("unexpected openrouter weight: %d", cfg.Routing.ProviderWeights["openrouter"])
	}
	if cfg.Routing.CircuitBreaker == nil || !cfg.Routing.CircuitBreaker.Enabled {
		t.Fatalf("circuit breaker should be enabled")
	}
	if cfg.Routing.CircuitBreaker.FailureThreshold != 5 {
		t.Fatalf("unexpected failure_threshold: %d", cfg.Routing.CircuitBreaker.FailureThreshold)
	}
}

func TestLoadRouterModelConfigFromTOML_ModelUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "router.toml")
	content := `
[model_updates]
enabled = true
startup_sync = true
interval_hours = 24
write_router_toml = true
default_new_model_active = true
removed_model_policy = "disable_auto_managed"
source_dir = "data/model_sources"
startup_delay_seconds = 0.25
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp router.toml: %v", err)
	}

	cfg, err := LoadRouterModelConfigFromTOML(path)
	if err != nil {
		t.Fatalf("LoadRouterModelConfigFromTOML() error = %v", err)
	}
	if cfg.ModelUpdates == nil {
		t.Fatalf("ModelUpdates should not be nil")
	}
	if !cfg.ModelUpdates.Enabled || !cfg.ModelUpdates.StartupSync || !cfg.ModelUpdates.WriteRouterTOML {
		t.Fatalf("unexpected boolean model update config: %+v", cfg.ModelUpdates)
	}
	if cfg.ModelUpdates.IntervalHours != 24 {
		t.Fatalf("IntervalHours = %d, want 24", cfg.ModelUpdates.IntervalHours)
	}
	if !cfg.ModelUpdates.DefaultNewModelActive {
		t.Fatalf("DefaultNewModelActive should be true")
	}
	if cfg.ModelUpdates.RemovedModelPolicy != "disable_auto_managed" {
		t.Fatalf("RemovedModelPolicy = %q", cfg.ModelUpdates.RemovedModelPolicy)
	}
	if cfg.ModelUpdates.SourceDir != "data/model_sources" {
		t.Fatalf("SourceDir = %q", cfg.ModelUpdates.SourceDir)
	}
	if cfg.ModelUpdates.StartupDelaySeconds != 0.25 {
		t.Fatalf("StartupDelaySeconds = %v", cfg.ModelUpdates.StartupDelaySeconds)
	}
}

func TestLoadRouterModelConfigFromTOML_ModelsOptional(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "router.toml")
	content := `
[[providers]]
name = "openai"
type = "openai"
api_key_env = "OPENAI_API_KEY"

[routing]
default_pair = "default"

[[routing.pairs]]
name = "default"
strong_model = "openai/gpt-4.1"
weak_model = "openai/gpt-4.1-mini"

[model_updates]
enabled = true
startup_sync = true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp router.toml: %v", err)
	}

	cfg, err := LoadRouterModelConfigFromTOML(path)
	if err != nil {
		t.Fatalf("LoadRouterModelConfigFromTOML() error = %v", err)
	}
	if len(cfg.Models) != 0 {
		t.Fatalf("Models length = %d, want 0", len(cfg.Models))
	}
	if cfg.Routing == nil || cfg.Routing.DefaultPair == nil || *cfg.Routing.DefaultPair != "default" {
		t.Fatalf("routing pair was not loaded: %+v", cfg.Routing)
	}
	if cfg.ModelUpdates == nil {
		t.Fatalf("ModelUpdates should not be nil")
	}
	if cfg.ModelUpdates.WriteRouterTOML {
		t.Fatalf("WriteRouterTOML should default to false when omitted")
	}
}

func TestLoadRouterModelConfigFromTOML_Logging(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "router.toml")
	content := `
[logging]
level = "warn"
format = "json"
stdout_enabled = false
file_path = "/tmp/llm-router.log"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp router.toml: %v", err)
	}

	cfg, err := LoadRouterModelConfigFromTOML(path)
	if err != nil {
		t.Fatalf("LoadRouterModelConfigFromTOML() error = %v", err)
	}
	if cfg.Logging == nil {
		t.Fatalf("Logging should not be nil")
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("Logging.Level = %q, want warn", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("Logging.Format = %q, want json", cfg.Logging.Format)
	}
	if cfg.Logging.StdoutEnabled {
		t.Fatalf("Logging.StdoutEnabled should be false")
	}
	if cfg.Logging.FilePath != "/tmp/llm-router.log" {
		t.Fatalf("Logging.FilePath = %q, want /tmp/llm-router.log", cfg.Logging.FilePath)
	}
}

func TestLoadRouterModelConfigFromTOML_QwenTTSPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "router.toml")
	content := `
[plugins.tts.qwen_tts]
command = "/usr/local/bin/qwen-tts-adapter"
args = ["--cache-dir", "./data/qwen-tts"]
working_dir = "/tmp/qwen-tts"
default_model = "qwen-tts-latest"
models = ["qwen-tts-latest", "qwen-tts-v1"]
timeout = 45
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp router.toml: %v", err)
	}

	cfg, err := LoadRouterModelConfigFromTOML(path)
	if err != nil {
		t.Fatalf("LoadRouterModelConfigFromTOML() error = %v", err)
	}
	if cfg.Plugins == nil {
		t.Fatalf("Plugins should not be nil")
	}
	qwenCfg, ok := cfg.Plugins.TTS["qwen_tts"]
	if !ok {
		t.Fatalf("expected qwen_tts plugin config, got %#v", cfg.Plugins.TTS)
	}
	if qwenCfg["command"] != "/usr/local/bin/qwen-tts-adapter" {
		t.Fatalf("command = %#v", qwenCfg["command"])
	}
	if qwenCfg["working_dir"] != "/tmp/qwen-tts" {
		t.Fatalf("working_dir = %#v", qwenCfg["working_dir"])
	}
	if qwenCfg["default_model"] != "qwen-tts-latest" {
		t.Fatalf("default_model = %#v", qwenCfg["default_model"])
	}
	models, ok := qwenCfg["models"].([]any)
	if !ok || len(models) != 2 {
		t.Fatalf("models parse failed: %#v", qwenCfg["models"])
	}
	if timeout, ok := numberAsInt64(qwenCfg["timeout"]); !ok || timeout != 45 {
		t.Fatalf("timeout = %#v, want 45", qwenCfg["timeout"])
	}
}
