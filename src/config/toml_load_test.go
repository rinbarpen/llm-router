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
