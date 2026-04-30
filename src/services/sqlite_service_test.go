package services_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rinbarpen/llm-router/src/config"
	"github.com/rinbarpen/llm-router/src/db"
	"github.com/rinbarpen/llm-router/src/migrate"
	"github.com/rinbarpen/llm-router/src/schemas"
	"github.com/rinbarpen/llm-router/src/services"
)

func TestCatalogServiceSQLiteCRUD(t *testing.T) {
	ctx := context.Background()
	store, err := db.Connect(ctx, filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer store.Close()
	if err := migrate.Bootstrap(ctx, store, config.Config{
		MigrateFromSQLite: false,
		ModelConfigPath:   filepath.Join(t.TempDir(), "missing-router.toml"),
	}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	svc := services.NewCatalogService(store)
	provider, err := svc.CreateProvider(ctx, schemas.ProviderCreate{
		Name:     "openai",
		Type:     "openai",
		BaseURL:  stringPtrForTest("https://api.openai.com/v1"),
		Settings: map[string]any{"api_key_env": "OPENAI_API_KEY"},
	})
	if err != nil {
		t.Fatalf("CreateProvider() error = %v", err)
	}
	if provider.ID == 0 || provider.Settings["api_key_env"] != "OPENAI_API_KEY" {
		t.Fatalf("unexpected provider: %#v", provider)
	}

	model, err := svc.CreateModel(ctx, schemas.ModelCreate{
		ProviderName:  "openai",
		Name:          "gpt-test",
		DefaultParams: map[string]any{"temperature": 0.2},
		Config:        map[string]any{"managed_by": "test"},
	})
	if err != nil {
		t.Fatalf("CreateModel() error = %v", err)
	}
	if model.ID == 0 || model.ProviderName != "openai" {
		t.Fatalf("unexpected model: %#v", model)
	}

	key, err := svc.CreateAPIKey(ctx, schemas.APIKeyCreate{
		Name:          stringPtrForTest("dev"),
		AllowedModels: []string{"gpt-test"},
	})
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if key.ID == 0 || key.Key == nil || key.Name == nil || *key.Name != "dev" {
		t.Fatalf("unexpected api key: %#v", key)
	}
}

func stringPtrForTest(v string) *string {
	return &v
}
