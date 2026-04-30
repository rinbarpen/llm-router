package services

import (
	"testing"

	"github.com/rinbarpen/llm-router/src/config"
)

func TestConfiguredProviderNamesDeduplicatesAndSkipsBlank(t *testing.T) {
	names := configuredProviderNames([]config.ProviderConfig{
		{Name: "openai"},
		{Name: "vapi"},
		{Name: "openai"},
		{Name: " "},
	})

	if len(names) != 2 {
		t.Fatalf("configuredProviderNames length = %d, want 2 (%#v)", len(names), names)
	}
	if names[0] != "openai" || names[1] != "vapi" {
		t.Fatalf("configuredProviderNames = %#v", names)
	}
}

func TestConfiguredModelProviderNamesSkipsBlank(t *testing.T) {
	names := configuredModelProviderNames([]config.ModelConfigEntry{
		{Provider: "openai", Name: "gpt-4.1"},
		{Provider: "openai", Name: "gpt-4.1-mini"},
		{Provider: "gemini", Name: "gemini-2.5-pro"},
		{Provider: " ", Name: "broken"},
	})

	if len(names) != 2 {
		t.Fatalf("configuredModelProviderNames length = %d, want 2 (%#v)", len(names), names)
	}
	if _, ok := names["openai"]; !ok {
		t.Fatalf("openai missing from %#v", names)
	}
	if _, ok := names["gemini"]; !ok {
		t.Fatalf("gemini missing from %#v", names)
	}
}

func TestShouldBackfillProviderModels(t *testing.T) {
	legacyProviders := map[string]struct{}{"gemini": {}}

	if shouldBackfillProviderModels(config.ProviderConfig{Name: "openai", IsActive: true}, 0, legacyProviders) != true {
		t.Fatalf("active provider with no DB models and no legacy models should backfill")
	}
	if shouldBackfillProviderModels(config.ProviderConfig{Name: "openai", IsActive: true}, 2, legacyProviders) {
		t.Fatalf("provider with existing DB models should not backfill")
	}
	if shouldBackfillProviderModels(config.ProviderConfig{Name: "gemini", IsActive: true}, 0, legacyProviders) {
		t.Fatalf("provider with legacy router.toml models should not backfill")
	}
	if shouldBackfillProviderModels(config.ProviderConfig{Name: "claude", IsActive: false}, 0, legacyProviders) {
		t.Fatalf("inactive provider should not backfill")
	}
}
