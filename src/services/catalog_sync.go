package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rinbarpen/llm-router/src/config"
	"github.com/rinbarpen/llm-router/src/db"
	"github.com/rinbarpen/llm-router/src/schemas"
)

// SyncRouterTOML upserts providers and models from router.toml into SQLite (OpenAI-style catalog).
func (s *CatalogService) SyncRouterTOML(ctx context.Context, configPath string) error {
	cfg, err := config.LoadRouterModelConfigFromTOML(configPath)
	if err != nil {
		return err
	}
	configuredProviderNames := configuredProviderNames(cfg.Providers)
	legacyModelProviders := configuredModelProviderNames(cfg.Models)
	sourceDir := "data/model_sources"
	defaultNewModelActive := false
	if cfg.ModelUpdates != nil {
		if strings.TrimSpace(cfg.ModelUpdates.SourceDir) != "" {
			sourceDir = cfg.ModelUpdates.SourceDir
		}
		defaultNewModelActive = cfg.ModelUpdates.DefaultNewModelActive
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
	for _, k := range cfg.APIKeys {
		resolved := k.ResolvedKey()
		if resolved == nil || strings.TrimSpace(*resolved) == "" {
			continue
		}
		if err := upsertAPIKeyFromConfig(ctx, tx, k, strings.TrimSpace(*resolved)); err != nil {
			return err
		}
	}
	if err := deleteStaleProviders(ctx, tx, configuredProviderNames); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit sync: %w", err)
	}
	if err := s.backfillMissingProviderModels(ctx, cfg.Providers, legacyModelProviders, sourceDir, defaultNewModelActive); err != nil {
		return err
	}
	return nil
}

func configuredProviderNames(items []config.ProviderConfig) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func configuredModelProviderNames(items []config.ModelConfigEntry) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Provider)
		if name == "" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}

func deleteStaleProviders(ctx context.Context, tx *db.Tx, keepNames []string) error {
	rows, err := tx.Query(ctx, `SELECT name FROM providers`)
	if err != nil {
		return fmt.Errorf("list providers for stale cleanup: %w", err)
	}
	defer rows.Close()

	stale := make([]string, 0)
	keep := make(map[string]struct{}, len(keepNames))
	for _, name := range keepNames {
		keep[name] = struct{}{}
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan provider for stale cleanup: %w", err)
		}
		if _, ok := keep[name]; ok {
			continue
		}
		stale = append(stale, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate providers for stale cleanup: %w", err)
	}
	if len(stale) == 0 {
		return nil
	}
	placeholders := makePlaceholders(len(stale))
	args := make([]any, len(stale))
	for i, name := range stale {
		args[i] = name
	}
	if _, err := tx.Exec(ctx, `DELETE FROM provider_model_catalog_cache WHERE provider_name IN (`+placeholders+`)`, args...); err != nil {
		return fmt.Errorf("delete stale provider catalog cache: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM providers WHERE name IN (`+placeholders+`)`, args...); err != nil {
		return fmt.Errorf("delete stale providers: %w", err)
	}
	return nil
}

func makePlaceholders(n int) string {
	items := make([]string, n)
	for i := range items {
		items[i] = "?"
	}
	return strings.Join(items, ",")
}

func upsertProviderFromConfig(ctx context.Context, tx *db.Tx, provider config.ProviderConfig) error {
	settings := providerSettingsForSync(provider)
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

func providerSettingsForSync(provider config.ProviderConfig) map[string]any {
	settings := provider.Settings
	if settings == nil {
		settings = map[string]any{}
	} else {
		settings = cloneAnyMap(settings)
	}
	if provider.APIKeyEnv != nil && strings.TrimSpace(*provider.APIKeyEnv) != "" {
		settings["api_key_env"] = strings.TrimSpace(*provider.APIKeyEnv)
	}
	return settings
}

func (s *CatalogService) backfillMissingProviderModels(ctx context.Context, providers []config.ProviderConfig, legacyProviders map[string]struct{}, sourceDir string, defaultNewModelActive bool) error {
	if s == nil {
		return nil
	}
	for _, provider := range providers {
		providerName := strings.TrimSpace(provider.Name)
		if providerName == "" {
			continue
		}
		existing, err := s.ListModelsByProvider(ctx, providerName)
		if err != nil {
			return fmt.Errorf("list models for provider %q: %w", providerName, err)
		}
		if !shouldBackfillProviderModels(provider, len(existing), legacyProviders) {
			continue
		}

		providerRow, err := s.GetProviderByName(ctx, providerName)
		if err != nil {
			return fmt.Errorf("load provider %q for model backfill: %w", providerName, err)
		}
		discovered, err := s.discoverProviderModels(ctx, providerRow, sourceDir)
		if err != nil {
			return fmt.Errorf("discover models for provider %q: %w", providerName, err)
		}
		if len(discovered) == 0 {
			continue
		}

		merged := MergeDiscoveredModels(providerName, nil, discovered, MergeModelOptions{
			DefaultNewModelActive: defaultNewModelActive,
			ManagedAt:             time.Now().UTC().Format(time.RFC3339),
		})
		if err := s.upsertDiscoveredModels(ctx, merged.Models); err != nil {
			return fmt.Errorf("backfill models for provider %q: %w", providerName, err)
		}
	}
	return nil
}

func shouldBackfillProviderModels(provider config.ProviderConfig, existingModelCount int, legacyProviders map[string]struct{}) bool {
	providerName := strings.TrimSpace(provider.Name)
	if providerName == "" || !provider.IsActive || existingModelCount > 0 {
		return false
	}
	_, hasLegacyModels := legacyProviders[providerName]
	return !hasLegacyModels
}

func (s *CatalogService) upsertDiscoveredModels(ctx context.Context, models []schemas.Model) error {
	if len(models) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin backfill tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, model := range models {
		entry := config.ModelConfigEntry{
			Name:             model.Name,
			Provider:         model.ProviderName,
			DisplayName:      model.DisplayName,
			Description:      model.Description,
			RemoteIdentifier: model.RemoteIdentifier,
			IsActive:         model.IsActive,
			DefaultParams:    model.DefaultParams,
			Config:           model.Config,
			DownloadURI:      model.DownloadURI,
			LocalPath:        model.LocalPath,
		}
		if tags := configStringSlice(model.Config["tags"]); len(tags) > 0 {
			entry.Tags = tags
		}
		if err := upsertModelFromConfig(ctx, tx, entry); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit backfill models: %w", err)
	}
	return nil
}

func upsertModelFromConfig(ctx context.Context, tx *db.Tx, m config.ModelConfigEntry) error {
	var providerID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM providers WHERE name = $1`, m.Provider).Scan(&providerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

func upsertAPIKeyFromConfig(ctx context.Context, tx *db.Tx, item config.APIKeyConfig, resolvedKey string) error {
	allowedModelsRaw, err := json.Marshal(item.AllowedModels)
	if err != nil {
		return fmt.Errorf("marshal api key allowed_models: %w", err)
	}
	allowedProvidersRaw, err := json.Marshal(item.AllowedProviders)
	if err != nil {
		return fmt.Errorf("marshal api key allowed_providers: %w", err)
	}
	ipAllowlistRaw, err := json.Marshal(item.IPAllowlist)
	if err != nil {
		return fmt.Errorf("marshal api key ip_allowlist: %w", err)
	}
	parameterLimits := map[string]any{}
	if item.ParameterLimits != nil {
		parameterLimits = map[string]any{
			"max_tokens":        item.ParameterLimits.MaxTokens,
			"temperature":       item.ParameterLimits.Temperature,
			"top_p":             item.ParameterLimits.TopP,
			"frequency_penalty": item.ParameterLimits.FrequencyPenalty,
			"presence_penalty":  item.ParameterLimits.PresencePenalty,
			"custom_limits":     item.ParameterLimits.CustomLimits,
		}
	}
	parameterLimitsRaw, err := json.Marshal(parameterLimits)
	if err != nil {
		return fmt.Errorf("marshal api key parameter_limits: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO api_keys(
			id, key, name, is_active, owner_type, expires_at, quota_tokens_monthly, ip_allowlist,
			allowed_models, allowed_providers, parameter_limits, created_at, updated_at
		)
		VALUES(
			(SELECT COALESCE(MAX(id), 0) + 1 FROM api_keys),
			$1,$2,$3,'system',$4,$5,$6::jsonb,$7::jsonb,$8::jsonb,$9::jsonb,now(),now()
		)
		ON CONFLICT (key) DO UPDATE SET
			name = EXCLUDED.name,
			is_active = EXCLUDED.is_active,
			owner_type = EXCLUDED.owner_type,
			expires_at = EXCLUDED.expires_at,
			quota_tokens_monthly = EXCLUDED.quota_tokens_monthly,
			ip_allowlist = EXCLUDED.ip_allowlist,
			allowed_models = EXCLUDED.allowed_models,
			allowed_providers = EXCLUDED.allowed_providers,
			parameter_limits = EXCLUDED.parameter_limits,
			updated_at = now()
	`, resolvedKey, item.Name, item.IsActive, item.ExpiresAt, item.QuotaTokensMonth, string(ipAllowlistRaw), string(allowedModelsRaw), string(allowedProvidersRaw), string(parameterLimitsRaw))
	if err != nil {
		return fmt.Errorf("upsert api key: %w", err)
	}
	return nil
}

// SyncRouterTOMLWithPool is a package-level helper for migrate/bootstrap (same logic as CatalogService.SyncRouterTOML).
func SyncRouterTOMLWithPool(ctx context.Context, pool *db.Store, configPath string) error {
	return NewCatalogService(pool).SyncRouterTOML(ctx, configPath)
}
