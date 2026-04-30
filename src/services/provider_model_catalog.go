package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rinbarpen/llm-router/src/db"
	"github.com/rinbarpen/llm-router/src/providers"
	"github.com/rinbarpen/llm-router/src/schemas"
)

var providerCatalogHTTPClient = &http.Client{Timeout: 20 * time.Second}

func (s *CatalogService) SyncProviderModelCatalog(ctx context.Context, providerName string) (map[string]any, error) {
	provider, err := s.GetProviderByName(ctx, providerName)
	if err != nil {
		return nil, err
	}
	models, err := s.fetchProviderModelsLive(ctx, provider, true)
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		metadataRaw, _ := json.Marshal(model["metadata"])
		_, _ = s.pool.Exec(ctx, `
			INSERT INTO provider_model_catalog_cache(provider_name, model_name, metadata, fetched_at)
			VALUES($1,$2,$3,now())
			ON CONFLICT(provider_name, model_name) DO UPDATE SET
				metadata = EXCLUDED.metadata,
				fetched_at = now()
		`, providerName, model["model_name"], metadataRaw)
	}
	return map[string]any{
		"provider_name": providerName,
		"count":         len(models),
		"synced_at":     time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *CatalogService) ListProviderSupportedModels(ctx context.Context, providerName string) ([]string, error) {
	provider, err := s.GetProviderByName(ctx, providerName)
	if err != nil {
		return nil, err
	}
	models, err := s.fetchProviderModelsLive(ctx, provider, false)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(models))
	for _, row := range models {
		name, _ := row["model_name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

func (s *CatalogService) ListProviderModelCatalog(ctx context.Context, providerName string) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT model_name, metadata, fetched_at
		FROM provider_model_catalog_cache
		WHERE provider_name = $1
		ORDER BY model_name ASC
	`, providerName)
	if err != nil {
		return nil, fmt.Errorf("list provider model catalog: %w", err)
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var (
			modelName string
			metadata  []byte
			fetchedAt time.Time
			metaMap   map[string]any
		)
		if err := rows.Scan(&modelName, &metadata, &fetchedAt); err != nil {
			return nil, fmt.Errorf("scan provider model catalog: %w", err)
		}
		_ = json.Unmarshal(metadata, &metaMap)
		out = append(out, map[string]any{
			"model_name": modelName,
			"metadata":   metaMap,
			"fetched_at": fetchedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, rows.Err()
}

func (s *CatalogService) ReconcileProviderModels(ctx context.Context, providerName string) (map[string]any, error) {
	localModels, err := s.ListModelsByProvider(ctx, providerName)
	if err != nil {
		return nil, err
	}
	cached, err := s.ListProviderModelCatalog(ctx, providerName)
	if err != nil {
		return nil, err
	}
	localSet := map[string]struct{}{}
	for _, m := range localModels {
		localSet[m.Name] = struct{}{}
	}
	cacheSet := map[string]struct{}{}
	for _, row := range cached {
		if name, ok := row["model_name"].(string); ok {
			cacheSet[name] = struct{}{}
		}
	}
	missingInLocal := make([]string, 0)
	missingInRemote := make([]string, 0)
	for name := range cacheSet {
		if _, ok := localSet[name]; !ok {
			missingInLocal = append(missingInLocal, name)
		}
	}
	for name := range localSet {
		if _, ok := cacheSet[name]; !ok {
			missingInRemote = append(missingInRemote, name)
		}
	}
	return map[string]any{
		"provider_name":      providerName,
		"missing_in_local":   missingInLocal,
		"missing_in_catalog": missingInRemote,
	}, nil
}

func (s *CatalogService) fetchProviderModels(ctx context.Context, provider schemas.Provider) ([]map[string]any, error) {
	pt := strings.ToLower(strings.TrimSpace(provider.Type))
	switch pt {
	case "openai", "openrouter", "grok", "groq", "deepseek", "siliconflow", "aihubmix":
		return s.fetchOpenAICompatibleModels(ctx, provider)
	case "gemini":
		return fetchGeminiModels(ctx, provider)
	case "claude", "anthropic":
		return fetchAnthropicModels(ctx, provider)
	default:
		return nil, fmt.Errorf("%w: provider type %s does not support live model discovery", ErrNotImplemented, provider.Type)
	}
}

func (s *CatalogService) fetchProviderModelsLive(ctx context.Context, provider schemas.Provider, refresh bool) ([]map[string]any, error) {
	if !refresh && s != nil && s.discoveryRT != nil {
		if cached, ok := s.discoveryRT.get(provider.Name); ok {
			return cached, nil
		}
	}
	models, err := s.fetchProviderModels(ctx, provider)
	if err != nil {
		return nil, err
	}
	if s != nil && s.discoveryRT != nil {
		s.discoveryRT.put(provider.Name, models)
	}
	return models, nil
}

func (s *CatalogService) fetchOpenAICompatibleModels(ctx context.Context, provider schemas.Provider) ([]map[string]any, error) {
	if !supportsOpenAICompatibleDiscovery(provider) {
		return nil, fmt.Errorf("%w: provider type %s does not support live model discovery", ErrNotImplemented, provider.Type)
	}
	ordered := discoveryOpenAICompatibleBaseURLs(provider)
	if len(ordered) == 0 {
		return nil, fmt.Errorf("%w: provider base_url is required for model sync", ErrNotImplemented)
	}

	var lastErr error
	for _, baseURL := range ordered {
		baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		start := time.Now()
		rows, err := fetchOpenAICompatibleModelsForBaseURL(ctx, provider, baseURL)
		retryable := isRetryableOpenAIError(err)
		if s != nil && s.endpointRT != nil {
			s.endpointRT.finish(provider.Name, baseURL, err == nil, time.Since(start), retryable)
		}
		if err == nil {
			return rows, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("%w: provider base_url is required for model sync", ErrNotImplemented)
}

func fetchOpenAICompatibleModelsForBaseURL(ctx context.Context, provider schemas.Provider, baseURL string) ([]map[string]any, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("provider base_url is required for model sync")
	}
	endpoint := openAICompatibleModelsEndpoint(provider, baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if apiKey := resolveProviderAPIKey(provider); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := providerCatalogHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("models endpoint status=%d: %s", resp.StatusCode, string(body))
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0)
	if data, ok := payload["data"].([]any); ok {
		for _, row := range data {
			m, ok := row.(map[string]any)
			if !ok {
				continue
			}
			id, _ := m["id"].(string)
			if strings.TrimSpace(id) == "" {
				continue
			}
			rows = append(rows, map[string]any{
				"model_name": id,
				"metadata":   m,
			})
		}
	}
	return rows, nil
}

func openAICompatibleModelsEndpoint(provider schemas.Provider, baseURL string) string {
	if provider.Settings != nil {
		if raw, ok := provider.Settings["models_endpoint"].(string); ok && strings.TrimSpace(raw) != "" {
			endpoint := strings.TrimSpace(raw)
			if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
				return endpoint
			}
			return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(endpoint, "/")
		}
	}
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.EqualFold(strings.TrimSpace(provider.Type), "qwen") && !strings.Contains(strings.ToLower(endpoint), "/compatible-mode/v1") {
		return endpoint + "/compatible-mode/v1/models"
	}
	if !strings.HasSuffix(strings.ToLower(endpoint), "/v1") {
		endpoint += "/v1"
	}
	return endpoint + "/models"
}

func fetchGeminiModels(ctx context.Context, provider schemas.Provider) ([]map[string]any, error) {
	baseURL := "https://generativelanguage.googleapis.com"
	if provider.BaseURL != nil && strings.TrimSpace(*provider.BaseURL) != "" {
		baseURL = strings.TrimRight(strings.TrimSpace(*provider.BaseURL), "/")
	}
	key := ""
	key = resolveProviderAPIKey(provider)
	if key == "" {
		return nil, fmt.Errorf("provider api key is required for gemini model sync")
	}
	url := baseURL + "/v1beta/models?key=" + key
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := providerCatalogHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini models endpoint status=%d: %s", resp.StatusCode, string(body))
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0)
	if data, ok := payload["models"].([]any); ok {
		for _, row := range data {
			m, ok := row.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			name = strings.TrimPrefix(name, "models/")
			if strings.TrimSpace(name) == "" {
				continue
			}
			rows = append(rows, map[string]any{
				"model_name": name,
				"metadata":   m,
			})
		}
	}
	return rows, nil
}

func fetchAnthropicModels(ctx context.Context, provider schemas.Provider) ([]map[string]any, error) {
	baseURL := "https://api.anthropic.com"
	if provider.BaseURL != nil && strings.TrimSpace(*provider.BaseURL) != "" {
		baseURL = strings.TrimRight(strings.TrimSpace(*provider.BaseURL), "/")
	}
	key := ""
	key = resolveProviderAPIKey(provider)
	if key == "" {
		return nil, fmt.Errorf("provider api key is required for anthropic model sync")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := providerCatalogHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic models endpoint status=%d: %s", resp.StatusCode, string(body))
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0)
	if data, ok := payload["data"].([]any); ok {
		for _, row := range data {
			m, ok := row.(map[string]any)
			if !ok {
				continue
			}
			id, _ := m["id"].(string)
			if strings.TrimSpace(id) == "" {
				continue
			}
			rows = append(rows, map[string]any{
				"model_name": id,
				"metadata":   m,
			})
		}
	}
	return rows, nil
}

func resolveProviderAPIKey(provider schemas.Provider) string {
	if provider.APIKey != nil {
		if key := strings.TrimSpace(*provider.APIKey); key != "" {
			return key
		}
	}
	if provider.Settings == nil {
		return ""
	}
	if raw, ok := provider.Settings["api_key"].(string); ok {
		if key := strings.TrimSpace(raw); key != "" {
			return key
		}
	}
	if raw, ok := provider.Settings["api_key_env"].(string); ok {
		if env := strings.TrimSpace(raw); env != "" {
			return strings.TrimSpace(os.Getenv(env))
		}
	}
	return ""
}

func supportsOpenAICompatibleDiscovery(provider schemas.Provider) bool {
	dbProvider := db.Provider{
		Type:     db.ProviderType(provider.Type),
		BaseURL:  provider.BaseURL,
		Settings: provider.Settings,
	}
	return providers.SupportsOpenAICompatibleModelDiscovery(dbProvider)
}

func resolveOpenAICompatibleDiscoveryBaseURL(provider schemas.Provider) string {
	dbProvider := db.Provider{
		Type:     db.ProviderType(provider.Type),
		BaseURL:  provider.BaseURL,
		Settings: provider.Settings,
	}
	base := providers.ResolveOpenAICompatibleBaseURL(dbProvider)
	base = strings.TrimSpace(base)
	if strings.EqualFold(base, "https://api.openai.com/v1") && !strings.EqualFold(provider.Type, "openai") {
		return ""
	}
	return base
}

func discoveryOpenAICompatibleBaseURLs(provider schemas.Provider) []string {
	out := make([]string, 0, 4)
	seen := map[string]struct{}{}
	appendURL := func(raw string) {
		item := strings.TrimRight(strings.TrimSpace(raw), "/")
		if item == "" {
			return
		}
		if _, ok := seen[item]; ok {
			return
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}

	if provider.Settings != nil {
		switch rows := provider.Settings["api_base_urls"].(type) {
		case []any:
			for _, item := range rows {
				if s, ok := item.(string); ok {
					appendURL(s)
				}
			}
		case []string:
			for _, item := range rows {
				appendURL(item)
			}
		case string:
			for _, item := range splitSettingCSV(rows) {
				appendURL(item)
			}
		}
		if raw, ok := provider.Settings["base_url"].(string); ok {
			appendURL(raw)
		}
	}
	if provider.BaseURL != nil {
		appendURL(*provider.BaseURL)
	}
	if len(out) == 0 {
		appendURL(resolveOpenAICompatibleDiscoveryBaseURL(provider))
	}
	return out
}

func IsLiveModelDiscoveryUnsupported(err error) bool {
	return errors.Is(err, ErrNotImplemented)
}
