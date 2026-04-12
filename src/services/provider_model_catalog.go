package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rinbarpen/llm-router/src/schemas"
)

func (s *CatalogService) SyncProviderModelCatalog(ctx context.Context, providerName string) (map[string]any, error) {
	provider, err := s.GetProviderByName(ctx, providerName)
	if err != nil {
		return nil, err
	}
	models, err := s.fetchProviderModels(ctx, provider)
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
	case "openai", "openrouter", "grok", "groq", "deepseek", "siliconflow", "aihubmix", "volcengine", "remote_http", "ollama":
		return fetchOpenAICompatibleModels(ctx, provider)
	case "gemini":
		return fetchGeminiModels(ctx, provider)
	case "claude", "anthropic":
		return fetchAnthropicModels(ctx, provider)
	default:
		return nil, fmt.Errorf("provider type %s is not supported for model sync", provider.Type)
	}
}

func fetchOpenAICompatibleModels(ctx context.Context, provider schemas.Provider) ([]map[string]any, error) {
	baseURL := ""
	if provider.BaseURL != nil {
		baseURL = strings.TrimRight(strings.TrimSpace(*provider.BaseURL), "/")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("provider base_url is required for model sync")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	if provider.APIKey != nil && strings.TrimSpace(*provider.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(*provider.APIKey))
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
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

func fetchGeminiModels(ctx context.Context, provider schemas.Provider) ([]map[string]any, error) {
	baseURL := "https://generativelanguage.googleapis.com"
	if provider.BaseURL != nil && strings.TrimSpace(*provider.BaseURL) != "" {
		baseURL = strings.TrimRight(strings.TrimSpace(*provider.BaseURL), "/")
	}
	key := ""
	if provider.APIKey != nil {
		key = strings.TrimSpace(*provider.APIKey)
	}
	if key == "" {
		return nil, fmt.Errorf("provider api key is required for gemini model sync")
	}
	url := baseURL + "/v1beta/models?key=" + key
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
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
	if provider.APIKey != nil {
		key = strings.TrimSpace(*provider.APIKey)
	}
	if key == "" {
		return nil, fmt.Errorf("provider api key is required for anthropic model sync")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
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
