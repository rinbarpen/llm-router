package config

import (
	"os"
	"strings"
	"time"
)

type ParameterLimits struct {
	MaxTokens        *int           `json:"max_tokens,omitempty" toml:"max_tokens"`
	Temperature      *float64       `json:"temperature,omitempty" toml:"temperature"`
	TopP             *float64       `json:"top_p,omitempty" toml:"top_p"`
	FrequencyPenalty *float64       `json:"frequency_penalty,omitempty" toml:"frequency_penalty"`
	PresencePenalty  *float64       `json:"presence_penalty,omitempty" toml:"presence_penalty"`
	CustomLimits     map[string]any `json:"custom_limits,omitempty" toml:"custom_limits"`
}

type APIKeyConfig struct {
	Key              *string          `json:"key,omitempty" toml:"key"`
	KeyEnv           *string          `json:"key_env,omitempty" toml:"key_env"`
	Name             *string          `json:"name,omitempty" toml:"name"`
	ExpiresAt        *time.Time       `json:"expires_at,omitempty" toml:"expires_at"`
	QuotaTokensMonth *int64           `json:"quota_tokens_monthly,omitempty" toml:"quota_tokens_monthly"`
	IPAllowlist      []string         `json:"ip_allowlist,omitempty" toml:"ip_allowlist"`
	AllowedModels    []string         `json:"allowed_models,omitempty" toml:"allowed_models"`
	AllowedProviders []string         `json:"allowed_providers,omitempty" toml:"allowed_providers"`
	ParameterLimits  *ParameterLimits `json:"parameter_limits,omitempty" toml:"parameter_limits"`
	IsActive         bool             `json:"is_active" toml:"is_active"`
}

func (c APIKeyConfig) ResolvedKeys() []string {
	if c.Key != nil && strings.TrimSpace(*c.Key) != "" {
		return splitCSV(*c.Key)
	}
	if c.KeyEnv != nil && strings.TrimSpace(*c.KeyEnv) != "" {
		envValue := os.Getenv(strings.TrimSpace(*c.KeyEnv))
		return splitCSV(envValue)
	}
	return nil
}

func (c APIKeyConfig) ResolvedKey() *string {
	keys := c.ResolvedKeys()
	if len(keys) == 0 {
		return nil
	}
	return &keys[0]
}

func (c APIKeyConfig) IsModelAllowed(providerName, modelName string) bool {
	if !c.IsActive {
		return false
	}
	if len(c.AllowedProviders) > 0 && !contains(c.AllowedProviders, providerName) {
		return false
	}
	if len(c.AllowedModels) == 0 {
		return true
	}
	fullName := providerName + "/" + modelName
	return contains(c.AllowedModels, modelName) || contains(c.AllowedModels, fullName)
}

func (c APIKeyConfig) ValidateParameters(parameters map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range parameters {
		out[k] = v
	}
	if c.ParameterLimits == nil {
		return out
	}
	limits := c.ParameterLimits
	if limits.MaxTokens != nil {
		out["max_tokens"] = minNumeric(out["max_tokens"], float64(*limits.MaxTokens))
	}
	if limits.Temperature != nil {
		out["temperature"] = minNumeric(out["temperature"], *limits.Temperature)
	}
	if limits.TopP != nil {
		out["top_p"] = minNumeric(out["top_p"], *limits.TopP)
	}
	if limits.FrequencyPenalty != nil {
		out["frequency_penalty"] = minNumeric(out["frequency_penalty"], *limits.FrequencyPenalty)
	}
	if limits.PresencePenalty != nil {
		out["presence_penalty"] = minNumeric(out["presence_penalty"], *limits.PresencePenalty)
	}
	return out
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token != "" {
			out = append(out, token)
		}
	}
	return out
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func minNumeric(current any, limit float64) any {
	switch v := current.(type) {
	case nil:
		return limit
	case int:
		if float64(v) > limit {
			return limit
		}
		return v
	case int32:
		if float64(v) > limit {
			return limit
		}
		return v
	case int64:
		if float64(v) > limit {
			return limit
		}
		return v
	case float32:
		if float64(v) > limit {
			return limit
		}
		return v
	case float64:
		if v > limit {
			return limit
		}
		return v
	default:
		return current
	}
}
