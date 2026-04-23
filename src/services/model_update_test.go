package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rinbarpen/llm-router/src/config"
	"github.com/rinbarpen/llm-router/src/schemas"
)

func TestLoadModelSourceFilesMatchesProviderNameAndType(t *testing.T) {
	dir := t.TempDir()
	source := `{
	  "provider_type": "claude",
	  "models": [
	    {
	      "id": "claude-sonnet-4-5-20250929",
	      "display_name": "Claude Sonnet 4.5",
	      "context_window": "200k",
	      "supports_vision": true,
	      "supports_tools": true,
	      "languages": ["en"],
	      "tags": ["chat", "claude"]
	    }
	  ]
	}`
	if err := os.WriteFile(filepath.Join(dir, "claude.json"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	models, err := LoadModelSourceFiles(dir, schemas.Provider{Name: "claude-main", Type: "claude"})
	if err != nil {
		t.Fatalf("LoadModelSourceFiles() error = %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("model count = %d, want 1", len(models))
	}
	if models[0].Name != "claude-sonnet-4-5-20250929" {
		t.Fatalf("Name = %q", models[0].Name)
	}
	if models[0].Config["context_window"] != "200k" || models[0].Config["supports_vision"] != true {
		t.Fatalf("unexpected config: %+v", models[0].Config)
	}
}

func TestMergeDiscoveredModelsPreservesManualFieldsAndDisablesNewModels(t *testing.T) {
	displayName := "Manual GPT"
	existing := []schemas.Model{{
		ProviderName: "openai",
		Name:         "gpt-existing",
		DisplayName:  &displayName,
		IsActive:     true,
		Config: map[string]any{
			"context_window": "manual",
			"tags":           []any{"manual"},
		},
	}}
	discovered := []DiscoveredModel{
		{
			Name:        "gpt-existing",
			DisplayName: "Official GPT",
			Config: map[string]any{
				"context_window": "128k",
				"supports_tools": true,
			},
			Tags: []string{"chat", "openai"},
		},
		{
			Name:        "gpt-new",
			DisplayName: "GPT New",
			Config: map[string]any{
				"context_window": "256k",
			},
			Tags: []string{"chat"},
		},
	}

	result := MergeDiscoveredModels("openai", existing, discovered, MergeModelOptions{
		DefaultNewModelActive: false,
		ManagedAt:             "2026-04-12T00:00:00Z",
	})

	if len(result.Models) != 2 {
		t.Fatalf("merged count = %d, want 2", len(result.Models))
	}
	if result.Models[0].DisplayName == nil || *result.Models[0].DisplayName != "Manual GPT" {
		t.Fatalf("manual display name was not preserved: %+v", result.Models[0].DisplayName)
	}
	if result.Models[0].Config["context_window"] != "manual" {
		t.Fatalf("manual config overwritten: %+v", result.Models[0].Config)
	}
	if result.Models[1].Name != "gpt-new" || result.Models[1].IsActive {
		t.Fatalf("new model should be present and inactive: %+v", result.Models[1])
	}
	if result.Models[1].Config["managed_by"] != ModelAutoUpdateManager {
		t.Fatalf("new model missing auto-managed marker: %+v", result.Models[1].Config)
	}
}

func TestReplaceAutoManagedModelBlockPreservesManualContent(t *testing.T) {
	input := `[server]
port = 18000

[[models]]
name = "manual"
provider = "openai"
display_name = "Manual"
`

	out, err := ReplaceAutoManagedModelBlock(input, []schemas.Model{{
		ProviderName: "openai",
		Name:         "gpt-new",
		IsActive:     false,
		Config:       map[string]any{"managed_by": ModelAutoUpdateManager, "context_window": "128k"},
	}})
	if err != nil {
		t.Fatalf("ReplaceAutoManagedModelBlock() error = %v", err)
	}
	if !strings.Contains(out, `name = "manual"`) {
		t.Fatalf("manual model removed:\n%s", out)
	}
	if !strings.Contains(out, AutoManagedModelsBeginMarker) || !strings.Contains(out, AutoManagedModelsEndMarker) {
		t.Fatalf("auto-managed markers missing:\n%s", out)
	}
	if !strings.Contains(out, `name = "gpt-new"`) || !strings.Contains(out, `is_active = false`) {
		t.Fatalf("auto-managed model missing:\n%s", out)
	}
	if strings.Contains(out, "\n[models]\n") {
		t.Fatalf("invalid [models] table emitted:\n%s", out)
	}

	second, err := ReplaceAutoManagedModelBlock(out, nil)
	if err != nil {
		t.Fatalf("ReplaceAutoManagedModelBlock(nil) error = %v", err)
	}
	if strings.Contains(second, `name = "gpt-new"`) {
		t.Fatalf("auto-managed model was not removed:\n%s", second)
	}
	if !strings.Contains(second, `name = "manual"`) {
		t.Fatalf("manual model removed after cleanup:\n%s", second)
	}
}

func TestReplaceAutoManagedModelBlockOutputParsesAsTOML(t *testing.T) {
	input := `[server]
port = 18000
`

	out, err := ReplaceAutoManagedModelBlock(input, []schemas.Model{{
		ProviderName: "openai",
		Name:         "gpt-new",
		IsActive:     true,
		Config: map[string]any{
			"managed_by":     ModelAutoUpdateManager,
			"context_window": "128k",
			"supports_tools": true,
		},
	}})
	if err != nil {
		t.Fatalf("ReplaceAutoManagedModelBlock() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "router.toml")
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatalf("write router.toml: %v", err)
	}
	if _, err := config.LoadRouterModelConfigFromTOML(path); err != nil {
		t.Fatalf("generated router.toml should parse, got %v\n%s", err, out)
	}
}

func TestModelUpdateStatusStoreRecordsRuns(t *testing.T) {
	store := NewModelUpdateStatusStore(2)
	store.Record(ModelUpdateRun{ProviderName: "openai", Added: []string{"gpt-new"}})
	store.Record(ModelUpdateRun{ProviderName: "gemini", Deleted: []string{"old"}})

	latest, ok := store.Latest()
	if !ok {
		t.Fatalf("Latest() should exist")
	}
	if latest.ProviderName != "gemini" {
		t.Fatalf("latest provider = %q", latest.ProviderName)
	}
	runs := store.Runs()
	if len(runs) != 2 {
		t.Fatalf("run count = %d", len(runs))
	}
}

func TestProviderSettingsForSyncIncludesAPIKeyEnv(t *testing.T) {
	apiKeyEnv := "VAPI_API_KEY"
	settings := providerSettingsForSync(config.ProviderConfig{
		Name:      "vapi",
		Type:      "openai",
		APIKeyEnv: &apiKeyEnv,
		Settings: map[string]any{
			"api_base_urls": []any{"https://api.vveai.com", "https://api.gpt.ge"},
		},
	})

	if settings["api_key_env"] != "VAPI_API_KEY" {
		t.Fatalf("api_key_env missing from synced settings: %+v", settings)
	}
}

func TestRunModelUpdateWithNoProvidersRecordsEmptyRun(t *testing.T) {
	store := NewModelUpdateStatusStore(5)
	out, err := RunModelUpdate(context.Background(), ModelUpdateDeps{
		ListProviders: func(context.Context) ([]schemas.Provider, error) {
			return nil, nil
		},
		StatusStore: store,
	}, ModelUpdateOptions{})
	if err != nil {
		t.Fatalf("RunModelUpdate() error = %v", err)
	}
	if len(out.ProviderRuns) != 0 {
		t.Fatalf("ProviderRuns count = %d", len(out.ProviderRuns))
	}
	latest, ok := store.Latest()
	if !ok || latest.ProviderName != "" {
		t.Fatalf("expected aggregate empty run, got ok=%v latest=%+v", ok, latest)
	}
}

func TestRunModelUpdateProviderFilterPreservesUnselectedManagedModels(t *testing.T) {
	var deleted []string
	var written []schemas.Model
	out, err := RunModelUpdate(context.Background(), ModelUpdateDeps{
		ListProviders: func(context.Context) ([]schemas.Provider, error) {
			return []schemas.Provider{
				{Name: "openrouter", Type: "openrouter"},
				{Name: "gemini", Type: "gemini"},
			}, nil
		},
		ListModelsByProvider: func(_ context.Context, providerName string) ([]schemas.Model, error) {
			return []schemas.Model{{
				ProviderName: providerName,
				Name:         providerName + "-old",
				IsActive:     true,
				Config:       map[string]any{"managed_by": ModelAutoUpdateManager},
			}}, nil
		},
		FetchProviderModels: func(_ context.Context, provider schemas.Provider) ([]DiscoveredModel, error) {
			if provider.Name != "openrouter" {
				t.Fatalf("unexpected fetch for provider %q", provider.Name)
			}
			return []DiscoveredModel{{Name: "openrouter-new"}}, nil
		},
		DeleteAutoManagedMissing: func(_ context.Context, _ string, names []string) error {
			deleted = append(deleted, names...)
			return nil
		},
		WriteRouterTOML: func(_ context.Context, models []schemas.Model) (string, error) {
			written = append(written, models...)
			return "backup.toml", nil
		},
	}, ModelUpdateOptions{
		ProviderFilters: []string{"openrouter"},
		WriteRouterTOML: true,
	})
	if err != nil {
		t.Fatalf("RunModelUpdate() error = %v", err)
	}
	if len(out.ProviderRuns) != 1 || out.ProviderRuns[0].ProviderName != "openrouter" {
		t.Fatalf("unexpected provider runs: %+v", out.ProviderRuns)
	}
	if len(deleted) != 1 || deleted[0] != "openrouter-old" {
		t.Fatalf("deleted = %+v", deleted)
	}
	if !containsModel(written, "openrouter", "openrouter-new") {
		t.Fatalf("selected provider model not written: %+v", written)
	}
	if !containsModel(written, "gemini", "gemini-old") {
		t.Fatalf("unselected provider auto-managed model not preserved: %+v", written)
	}
}

func TestRunModelUpdateUnsupportedProviderWithStaticSourceMerges(t *testing.T) {
	dir := t.TempDir()
	source := `{"provider_type":"local_cli","models":[{"id":"default","display_name":"Local Default"}]}`
	if err := os.WriteFile(filepath.Join(dir, "local_cli.json"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	provider := schemas.Provider{Name: "local", Type: "local_cli"}
	discovered, err := (&CatalogService{}).discoverProviderModels(context.Background(), provider, dir)
	if err != nil {
		t.Fatalf("discoverProviderModels() error = %v", err)
	}
	if len(discovered) != 1 || discovered[0].Name != "default" {
		t.Fatalf("unexpected discovered models: %+v", discovered)
	}
}

func TestRunModelUpdateUnsupportedProviderNoSourceSkipsWithoutDelete(t *testing.T) {
	var deleted []string
	out, err := RunModelUpdate(context.Background(), ModelUpdateDeps{
		ListProviders: func(context.Context) ([]schemas.Provider, error) {
			return []schemas.Provider{{Name: "local", Type: "local_cli"}}, nil
		},
		ListModelsByProvider: func(context.Context, string) ([]schemas.Model, error) {
			return []schemas.Model{{
				ProviderName: "local",
				Name:         "old",
				Config:       map[string]any{"managed_by": ModelAutoUpdateManager},
			}}, nil
		},
		FetchProviderModels: func(context.Context, schemas.Provider) ([]DiscoveredModel, error) {
			return nil, nil
		},
		DeleteAutoManagedMissing: func(_ context.Context, _ string, names []string) error {
			deleted = append(deleted, names...)
			return nil
		},
	}, ModelUpdateOptions{})
	if err != nil {
		t.Fatalf("RunModelUpdate() error = %v", err)
	}
	if len(out.ProviderRuns) != 1 || len(out.ProviderRuns[0].Skipped) != 1 {
		t.Fatalf("expected skipped run, got %+v", out.ProviderRuns)
	}
	if len(deleted) != 0 {
		t.Fatalf("skipped provider should not delete models, deleted=%+v", deleted)
	}
}

func containsModel(models []schemas.Model, providerName string, name string) bool {
	for _, model := range models {
		if model.ProviderName == providerName && model.Name == name {
			return true
		}
	}
	return false
}
