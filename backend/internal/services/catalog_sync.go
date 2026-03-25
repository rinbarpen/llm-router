package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rinbarpen/llm-router/backend/internal/config"
)

// SyncRouterTOML upserts providers and models from router.toml into PostgreSQL (OpenAI-style catalog).
func (s *CatalogService) SyncRouterTOML(ctx context.Context, configPath string) error {
	cfg, err := config.LoadRouterModelConfigFromTOML(configPath)
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, p := range cfg.Providers {
		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Type) == "" {
			continue
		}
		if err := upsertProviderFromConfig(ctx, tx, p); err != nil {
			return err
		}
	}

	for _, m := range cfg.Models {
		if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Provider) == "" {
			continue
		}
		if err := upsertModelFromConfig(ctx, tx, m); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit sync: %w", err)
	}
	return nil
}

func upsertProviderFromConfig(ctx context.Context, tx pgx.Tx, provider config.ProviderConfig) error {
	settings := provider.Settings
	if settings == nil {
		settings = map[string]any{}
	}
	settingsRaw, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal settings for %s: %w", provider.Name, err)
	}
	apiKey := provider.ResolvedAPIKey()
	var baseURL any
	if provider.BaseURL != nil {
		baseURL = *provider.BaseURL
	}
	var apiKeyVal any
	if apiKey != nil {
		apiKeyVal = *apiKey
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO providers(id, name, type, is_active, base_url, api_key, settings, created_at, updated_at)
		VALUES (
			(SELECT COALESCE(MAX(id), 0) + 1 FROM providers),
			$1,$2,$3,$4,$5,$6::jsonb,now(),now()
		)
		ON CONFLICT (name) DO UPDATE SET
			type = EXCLUDED.type,
			is_active = EXCLUDED.is_active,
			base_url = EXCLUDED.base_url,
			api_key = EXCLUDED.api_key,
			settings = EXCLUDED.settings,
			updated_at = now()
	`, provider.Name, provider.Type, provider.IsActive, baseURL, apiKeyVal, string(settingsRaw))
	if err != nil {
		return fmt.Errorf("upsert provider %q: %w", provider.Name, err)
	}
	return nil
}

func upsertModelFromConfig(ctx context.Context, tx pgx.Tx, m config.ModelConfigEntry) error {
	var providerID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM providers WHERE name = $1`, m.Provider).Scan(&providerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("model %q: unknown provider %q", m.Name, m.Provider)
		}
		return fmt.Errorf("lookup provider %q: %w", m.Provider, err)
	}

	defaultParams := m.DefaultParams
	if defaultParams == nil {
		defaultParams = map[string]any{}
	}
	modelCfg := cloneAnyMap(m.Config)
	if modelCfg == nil {
		modelCfg = map[string]any{}
	}
	if len(m.Tags) > 0 {
		tagsAny := make([]any, len(m.Tags))
		for i, t := range m.Tags {
			tagsAny[i] = t
		}
		modelCfg["tags"] = tagsAny
	}

	defaultRaw, err := json.Marshal(defaultParams)
	if err != nil {
		return fmt.Errorf("model %q default_params: %w", m.Name, err)
	}
	configRaw, err := json.Marshal(modelCfg)
	if err != nil {
		return fmt.Errorf("model %q config: %w", m.Name, err)
	}

	var displayName, description, remoteID, downloadURI, localPath any
	if m.DisplayName != nil {
		displayName = *m.DisplayName
	}
	if m.Description != nil {
		description = *m.Description
	}
	if m.RemoteIdentifier != nil {
		remoteID = *m.RemoteIdentifier
	}
	if m.DownloadURI != nil {
		downloadURI = *m.DownloadURI
	}
	if m.LocalPath != nil {
		localPath = *m.LocalPath
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO models (
			id, provider_id, name, display_name, description, is_active,
			remote_identifier, default_params, config, download_uri, local_path, created_at, updated_at
		)
		VALUES (
			(SELECT COALESCE(MAX(id), 0) + 1 FROM models),
			$1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10,now(),now()
		)
		ON CONFLICT (provider_id, name) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			description = EXCLUDED.description,
			is_active = EXCLUDED.is_active,
			remote_identifier = EXCLUDED.remote_identifier,
			default_params = EXCLUDED.default_params,
			config = EXCLUDED.config,
			download_uri = EXCLUDED.download_uri,
			local_path = EXCLUDED.local_path,
			updated_at = now()
	`, providerID, m.Name, displayName, description, m.IsActive, remoteID, string(defaultRaw), string(configRaw), downloadURI, localPath)
	if err != nil {
		return fmt.Errorf("upsert model %q/%q: %w", m.Provider, m.Name, err)
	}
	return nil
}

func cloneAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// SyncRouterTOMLWithPool is a package-level helper for migrate/bootstrap (same logic as CatalogService.SyncRouterTOML).
func SyncRouterTOMLWithPool(ctx context.Context, pool *pgxpool.Pool, configPath string) error {
	return NewCatalogService(pool).SyncRouterTOML(ctx, configPath)
}
