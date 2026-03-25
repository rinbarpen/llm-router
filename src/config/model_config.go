package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type ProviderConfig struct {
	Name      string         `json:"name" toml:"name"`
	Type      string         `json:"type" toml:"type"`
	BaseURL   *string        `json:"base_url,omitempty" toml:"base_url"`
	APIKey    *string        `json:"api_key,omitempty" toml:"api_key"`
	APIKeyEnv *string        `json:"api_key_env,omitempty" toml:"api_key_env"`
	IsActive  bool           `json:"is_active" toml:"is_active"`
	Settings  map[string]any `json:"settings,omitempty" toml:"settings"`
}

func (c ProviderConfig) ResolvedAPIKeys() []string {
	if c.APIKey != nil && strings.TrimSpace(*c.APIKey) != "" {
		return splitCSV(*c.APIKey)
	}
	if c.APIKeyEnv != nil && strings.TrimSpace(*c.APIKeyEnv) != "" {
		return splitCSV(os.Getenv(strings.TrimSpace(*c.APIKeyEnv)))
	}
	return nil
}

func (c ProviderConfig) ResolvedAPIKey() *string {
	keys := c.ResolvedAPIKeys()
	if len(keys) == 0 {
		return nil
	}
	return &keys[0]
}

type RateLimitEntry struct {
	MaxRequests int64          `json:"max_requests" toml:"max_requests"`
	PerSeconds  int64          `json:"per_seconds" toml:"per_seconds"`
	BurstSize   *int64         `json:"burst_size,omitempty" toml:"burst_size"`
	Notes       *string        `json:"notes,omitempty" toml:"notes"`
	Config      map[string]any `json:"config,omitempty" toml:"config"`
}

type ModelConfigEntry struct {
	Name             string          `json:"name" toml:"name"`
	Provider         string          `json:"provider" toml:"provider"`
	DisplayName      *string         `json:"display_name,omitempty" toml:"display_name"`
	Description      *string         `json:"description,omitempty" toml:"description"`
	RemoteIdentifier *string         `json:"remote_identifier,omitempty" toml:"remote_identifier"`
	IsActive         bool            `json:"is_active" toml:"is_active"`
	Tags             []string        `json:"tags,omitempty" toml:"tags"`
	DefaultParams    map[string]any  `json:"default_params,omitempty" toml:"default_params"`
	Config           map[string]any  `json:"config,omitempty" toml:"config"`
	DownloadURI      *string         `json:"download_uri,omitempty" toml:"download_uri"`
	LocalPath        *string         `json:"local_path,omitempty" toml:"local_path"`
	RateLimit        *RateLimitEntry `json:"rate_limit,omitempty" toml:"rate_limit"`
}

type ServerConfig struct {
	Host                  *string `json:"host,omitempty" toml:"host"`
	Port                  *int    `json:"port,omitempty" toml:"port"`
	AllowLocalWithoutAuth *bool   `json:"allow_local_without_auth,omitempty" toml:"allow_local_without_auth"`
}

type MonitorConfig struct {
	Port       *int    `json:"port,omitempty" toml:"port"`
	APIURL     *string `json:"api_url,omitempty" toml:"api_url"`
	APIBaseURL *string `json:"api_base_url,omitempty" toml:"api_base_url"`
}

type RoutingPairConfig struct {
	Name        string `json:"name" toml:"name"`
	StrongModel string `json:"strong_model" toml:"strong_model"`
	WeakModel   string `json:"weak_model" toml:"weak_model"`
}

type RoutingConfig struct {
	AnalyzerModel      *string             `json:"analyzer_model,omitempty" toml:"analyzer_model"`
	DefaultStrongModel *string             `json:"default_strong_model,omitempty" toml:"default_strong_model"`
	DefaultWeakModel   *string             `json:"default_weak_model,omitempty" toml:"default_weak_model"`
	DefaultPair        *string             `json:"default_pair,omitempty" toml:"default_pair"`
	Pairs              []RoutingPairConfig `json:"pairs,omitempty" toml:"pairs"`
	AnalyzerTimeoutMS  int                 `json:"analyzer_timeout_ms" toml:"analyzer_timeout_ms"`
	AutoFallbackMode   string              `json:"auto_fallback_mode" toml:"auto_fallback_mode"`
}

type PluginsConfig struct {
	TTS map[string]map[string]any `json:"tts,omitempty" toml:"tts"`
	ASR map[string]map[string]any `json:"asr,omitempty" toml:"asr"`
}

type RouterModelConfig struct {
	Providers []ProviderConfig   `json:"providers,omitempty" toml:"providers"`
	Models    []ModelConfigEntry `json:"models,omitempty" toml:"models"`
	APIKeys   []APIKeyConfig     `json:"api_keys,omitempty" toml:"api_keys"`
	Server    *ServerConfig      `json:"server,omitempty" toml:"server"`
	Monitor   *MonitorConfig     `json:"monitor,omitempty" toml:"monitor"`
	Routing   *RoutingConfig     `json:"routing,omitempty" toml:"routing"`
	Plugins   *PluginsConfig     `json:"plugins,omitempty" toml:"plugins"`
}

func (c *RouterModelConfig) Normalize() {
	for i := range c.Providers {
		if c.Providers[i].Settings == nil {
			c.Providers[i].Settings = map[string]any{}
		}
	}
	for i := range c.Models {
		if c.Models[i].DefaultParams == nil {
			c.Models[i].DefaultParams = map[string]any{}
		}
		if c.Models[i].Config == nil {
			c.Models[i].Config = map[string]any{}
		}
	}
	for i := range c.APIKeys {
		if !c.APIKeys[i].IsActive {
			continue
		}
	}
	if c.Routing != nil {
		if c.Routing.AnalyzerTimeoutMS == 0 {
			c.Routing.AnalyzerTimeoutMS = 1500
		}
		if strings.TrimSpace(c.Routing.AutoFallbackMode) == "" {
			c.Routing.AutoFallbackMode = "weak"
		}
	}
}

// LoadModelConfig currently supports JSON payloads and returns a clear error for TOML.
// This keeps Go backend behavior explicit until a dedicated TOML parser is added.
func LoadModelConfig(path string) (RouterModelConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return RouterModelConfig{}, fmt.Errorf("read config file: %w", err)
	}
	var cfg RouterModelConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return RouterModelConfig{}, fmt.Errorf("unsupported config format for %s (expected JSON in Go backend): %w", path, err)
	}
	cfg.Normalize()
	return cfg, nil
}
