package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "modernc.org/sqlite"

	"github.com/rinbarpen/llm-router/backend/internal/db"
	"github.com/rinbarpen/llm-router/backend/internal/schemas"
)

var ErrNotFound = errors.New("resource not found")
var ErrNotImplemented = errors.New("not implemented")

type StreamResponse struct {
	Body        io.ReadCloser
	ContentType string
}

type UpstreamStatusError struct {
	StatusCode int
	Detail     string
}

func (e *UpstreamStatusError) Error() string {
	if e == nil {
		return "upstream request failed"
	}
	return fmt.Sprintf("upstream error status=%d detail=%s", e.StatusCode, e.Detail)
}

type CatalogService struct {
	pool *pgxpool.Pool
}

func NewCatalogService(pool *pgxpool.Pool) *CatalogService {
	return &CatalogService{pool: pool}
}

func (s *CatalogService) ListProviders(ctx context.Context) ([]schemas.Provider, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, type, is_active, base_url, api_key, settings, created_at, updated_at
		FROM providers
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list providers query: %w", err)
	}
	defer rows.Close()

	providers := make([]schemas.Provider, 0)
	for rows.Next() {
		var (
			p           schemas.Provider
			settingsRaw []byte
		)
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.IsActive, &p.BaseURL, &p.APIKey, &settingsRaw, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		if len(settingsRaw) > 0 {
			_ = json.Unmarshal(settingsRaw, &p.Settings)
		}
		providers = append(providers, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate providers: %w", err)
	}
	return providers, nil
}

func (s *CatalogService) GetProviderByName(ctx context.Context, name string) (schemas.Provider, error) {
	var (
		item        schemas.Provider
		settingsRaw []byte
	)
	if err := s.pool.QueryRow(ctx, `
		SELECT id, name, type, is_active, base_url, api_key, settings, created_at, updated_at
		FROM providers WHERE name = $1
	`, name).Scan(
		&item.ID, &item.Name, &item.Type, &item.IsActive, &item.BaseURL, &item.APIKey, &settingsRaw, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return schemas.Provider{}, ErrNotFound
		}
		return schemas.Provider{}, fmt.Errorf("get provider by name: %w", err)
	}
	if len(settingsRaw) > 0 {
		_ = json.Unmarshal(settingsRaw, &item.Settings)
	}
	return item, nil
}

func (s *CatalogService) CreateProvider(ctx context.Context, in schemas.ProviderCreate) (schemas.Provider, error) {
	settings := in.Settings
	if settings == nil {
		settings = map[string]any{}
	}
	settingsRaw, err := json.Marshal(settings)
	if err != nil {
		return schemas.Provider{}, fmt.Errorf("marshal provider settings: %w", err)
	}

	var out schemas.Provider
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO providers(name, type, is_active, base_url, api_key, settings, created_at, updated_at)
		VALUES ($1,$2,true,$3,$4,$5,now(),now())
		RETURNING id, name, type, is_active, base_url, api_key, settings, created_at, updated_at
	`, in.Name, in.Type, in.BaseURL, in.APIKey, settingsRaw).Scan(
		&out.ID, &out.Name, &out.Type, &out.IsActive, &out.BaseURL, &out.APIKey, &settingsRaw, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		return schemas.Provider{}, fmt.Errorf("create provider: %w", err)
	}
	if len(settingsRaw) > 0 {
		_ = json.Unmarshal(settingsRaw, &out.Settings)
	}
	return out, nil
}

func (s *CatalogService) UpdateProvider(ctx context.Context, name string, in schemas.ProviderUpdate) (schemas.Provider, error) {
	current, err := s.GetProviderByName(ctx, name)
	if err != nil {
		return schemas.Provider{}, err
	}
	baseURL := current.BaseURL
	if in.BaseURL != nil {
		baseURL = in.BaseURL
	}
	apiKey := current.APIKey
	if in.APIKey != nil {
		apiKey = in.APIKey
	}
	isActive := current.IsActive
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	settings := current.Settings
	if in.Settings != nil {
		settings = in.Settings
	}
	if settings == nil {
		settings = map[string]any{}
	}
	settingsRaw, err := json.Marshal(settings)
	if err != nil {
		return schemas.Provider{}, fmt.Errorf("marshal provider settings: %w", err)
	}

	var out schemas.Provider
	if err := s.pool.QueryRow(ctx, `
		UPDATE providers
		SET base_url = $2, api_key = $3, is_active = $4, settings = $5, updated_at = now()
		WHERE name = $1
		RETURNING id, name, type, is_active, base_url, api_key, settings, created_at, updated_at
	`, name, baseURL, apiKey, isActive, settingsRaw).Scan(
		&out.ID, &out.Name, &out.Type, &out.IsActive, &out.BaseURL, &out.APIKey, &settingsRaw, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return schemas.Provider{}, ErrNotFound
		}
		return schemas.Provider{}, fmt.Errorf("update provider: %w", err)
	}
	_ = json.Unmarshal(settingsRaw, &out.Settings)
	return out, nil
}

func (s *CatalogService) ListModels(ctx context.Context) ([]schemas.Model, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.provider_id, p.name, m.name, m.display_name, m.description,
		       m.is_active, m.remote_identifier, m.default_params, m.config,
		       m.download_uri, m.local_path, m.created_at, m.updated_at
		FROM models m
		JOIN providers p ON p.id = m.provider_id
		ORDER BY m.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list models query: %w", err)
	}
	defer rows.Close()

	models := make([]schemas.Model, 0)
	for rows.Next() {
		var (
			m                schemas.Model
			defaultParamsRaw []byte
			configRaw        []byte
		)
		if err := rows.Scan(
			&m.ID, &m.ProviderID, &m.ProviderName, &m.Name, &m.DisplayName, &m.Description,
			&m.IsActive, &m.RemoteIdentifier, &defaultParamsRaw, &configRaw,
			&m.DownloadURI, &m.LocalPath, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		if len(defaultParamsRaw) > 0 {
			_ = json.Unmarshal(defaultParamsRaw, &m.DefaultParams)
		}
		if len(configRaw) > 0 {
			_ = json.Unmarshal(configRaw, &m.Config)
		}
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate models: %w", err)
	}
	return models, nil
}

func (s *CatalogService) CreateModel(ctx context.Context, in schemas.ModelCreate) (schemas.Model, error) {
	defaultParams := in.DefaultParams
	if defaultParams == nil {
		defaultParams = map[string]any{}
	}
	cfg := in.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	defaultRaw, err := json.Marshal(defaultParams)
	if err != nil {
		return schemas.Model{}, fmt.Errorf("marshal default params: %w", err)
	}
	configRaw, err := json.Marshal(cfg)
	if err != nil {
		return schemas.Model{}, fmt.Errorf("marshal model config: %w", err)
	}

	var providerID int64
	if err := s.pool.QueryRow(ctx, `SELECT id FROM providers WHERE name = $1`, in.ProviderName).Scan(&providerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return schemas.Model{}, ErrNotFound
		}
		return schemas.Model{}, fmt.Errorf("query provider by name: %w", err)
	}

	var out schemas.Model
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO models (
			provider_id, name, display_name, description, is_active,
			remote_identifier, default_params, config, download_uri, local_path, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,true,$5,$6,$7,$8,$9,now(),now())
		RETURNING id, provider_id, name, display_name, description, is_active,
		          remote_identifier, default_params, config, download_uri, local_path, created_at, updated_at
	`, providerID, in.Name, in.DisplayName, in.Description, in.RemoteIdentifier, defaultRaw, configRaw, in.DownloadURI, in.LocalPath).Scan(
		&out.ID, &out.ProviderID, &out.Name, &out.DisplayName, &out.Description, &out.IsActive,
		&out.RemoteIdentifier, &defaultRaw, &configRaw, &out.DownloadURI, &out.LocalPath, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		return schemas.Model{}, fmt.Errorf("create model: %w", err)
	}
	out.ProviderName = in.ProviderName
	if len(defaultRaw) > 0 {
		_ = json.Unmarshal(defaultRaw, &out.DefaultParams)
	}
	if len(configRaw) > 0 {
		_ = json.Unmarshal(configRaw, &out.Config)
	}
	return out, nil
}

func (s *CatalogService) GetModelByProviderAndName(ctx context.Context, providerName string, modelName string) (schemas.Model, error) {
	var (
		item             schemas.Model
		defaultParamsRaw []byte
		configRaw        []byte
	)
	if err := s.pool.QueryRow(ctx, `
		SELECT m.id, m.provider_id, p.name, m.name, m.display_name, m.description,
		       m.is_active, m.remote_identifier, m.default_params, m.config,
		       m.download_uri, m.local_path, m.created_at, m.updated_at
		FROM models m
		JOIN providers p ON p.id = m.provider_id
		WHERE p.name = $1 AND m.name = $2
	`, providerName, modelName).Scan(
		&item.ID, &item.ProviderID, &item.ProviderName, &item.Name, &item.DisplayName, &item.Description,
		&item.IsActive, &item.RemoteIdentifier, &defaultParamsRaw, &configRaw,
		&item.DownloadURI, &item.LocalPath, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return schemas.Model{}, ErrNotFound
		}
		return schemas.Model{}, fmt.Errorf("get model by provider and name: %w", err)
	}
	_ = json.Unmarshal(defaultParamsRaw, &item.DefaultParams)
	_ = json.Unmarshal(configRaw, &item.Config)
	return item, nil
}

func (s *CatalogService) UpdateModel(ctx context.Context, providerName string, modelName string, in schemas.ModelUpdate) (schemas.Model, error) {
	current, err := s.GetModelByProviderAndName(ctx, providerName, modelName)
	if err != nil {
		return schemas.Model{}, err
	}
	displayName := current.DisplayName
	if in.DisplayName != nil {
		displayName = in.DisplayName
	}
	description := current.Description
	if in.Description != nil {
		description = in.Description
	}
	isActive := current.IsActive
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	remoteIdentifier := current.RemoteIdentifier
	if in.RemoteIdentifier != nil {
		remoteIdentifier = in.RemoteIdentifier
	}
	defaultParams := current.DefaultParams
	if in.DefaultParams != nil {
		defaultParams = in.DefaultParams
	}
	config := current.Config
	if in.Config != nil {
		config = in.Config
	}
	downloadURI := current.DownloadURI
	if in.DownloadURI != nil {
		downloadURI = in.DownloadURI
	}
	localPath := current.LocalPath
	if in.LocalPath != nil {
		localPath = in.LocalPath
	}
	if defaultParams == nil {
		defaultParams = map[string]any{}
	}
	if config == nil {
		config = map[string]any{}
	}
	defaultParamsRaw, _ := json.Marshal(defaultParams)
	configRaw, _ := json.Marshal(config)

	var out schemas.Model
	if err := s.pool.QueryRow(ctx, `
		UPDATE models
		SET display_name = $3, description = $4, is_active = $5, remote_identifier = $6,
			default_params = $7, config = $8, download_uri = $9, local_path = $10, updated_at = now()
		WHERE provider_id = (
			SELECT id FROM providers WHERE name = $1
		) AND name = $2
		RETURNING id, provider_id, name, display_name, description, is_active,
		          remote_identifier, default_params, config, download_uri, local_path, created_at, updated_at
	`, providerName, modelName, displayName, description, isActive, remoteIdentifier, defaultParamsRaw, configRaw, downloadURI, localPath).Scan(
		&out.ID, &out.ProviderID, &out.Name, &out.DisplayName, &out.Description, &out.IsActive,
		&out.RemoteIdentifier, &defaultParamsRaw, &configRaw, &out.DownloadURI, &out.LocalPath, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return schemas.Model{}, ErrNotFound
		}
		return schemas.Model{}, fmt.Errorf("update model: %w", err)
	}
	out.ProviderName = providerName
	_ = json.Unmarshal(defaultParamsRaw, &out.DefaultParams)
	_ = json.Unmarshal(configRaw, &out.Config)
	return out, nil
}

func (s *CatalogService) ListModelsByProvider(ctx context.Context, providerName string) ([]schemas.Model, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.provider_id, p.name, m.name, m.display_name, m.description,
		       m.is_active, m.remote_identifier, m.default_params, m.config,
		       m.download_uri, m.local_path, m.created_at, m.updated_at
		FROM models m
		JOIN providers p ON p.id = m.provider_id
		WHERE p.name = $1
		ORDER BY m.id ASC
	`, providerName)
	if err != nil {
		return nil, fmt.Errorf("list models by provider query: %w", err)
	}
	defer rows.Close()

	models := make([]schemas.Model, 0)
	for rows.Next() {
		var (
			m                schemas.Model
			defaultParamsRaw []byte
			configRaw        []byte
		)
		if err := rows.Scan(
			&m.ID, &m.ProviderID, &m.ProviderName, &m.Name, &m.DisplayName, &m.Description,
			&m.IsActive, &m.RemoteIdentifier, &defaultParamsRaw, &configRaw,
			&m.DownloadURI, &m.LocalPath, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan model by provider: %w", err)
		}
		if len(defaultParamsRaw) > 0 {
			_ = json.Unmarshal(defaultParamsRaw, &m.DefaultParams)
		}
		if len(configRaw) > 0 {
			_ = json.Unmarshal(configRaw, &m.Config)
		}
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate models by provider: %w", err)
	}
	return models, nil
}

func (s *CatalogService) ListAPIKeys(ctx context.Context, includeInactive bool) ([]schemas.APIKey, error) {
	query := `
		SELECT id, key, name, is_active, allowed_models, allowed_providers, parameter_limits, created_at, updated_at
		FROM api_keys
	`
	args := []any{}
	if !includeInactive {
		query += ` WHERE is_active = true`
	}
	query += ` ORDER BY id ASC`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api keys query: %w", err)
	}
	defer rows.Close()

	out := make([]schemas.APIKey, 0)
	for rows.Next() {
		var (
			item                schemas.APIKey
			allowedModelsRaw    []byte
			allowedProvidersRaw []byte
			parameterLimitsRaw  []byte
		)
		if err := rows.Scan(
			&item.ID, &item.Key, &item.Name, &item.IsActive,
			&allowedModelsRaw, &allowedProvidersRaw, &parameterLimitsRaw, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		if len(allowedModelsRaw) > 0 {
			_ = json.Unmarshal(allowedModelsRaw, &item.AllowedModels)
		}
		if len(allowedProvidersRaw) > 0 {
			_ = json.Unmarshal(allowedProvidersRaw, &item.AllowedProviders)
		}
		if len(parameterLimitsRaw) > 0 {
			_ = json.Unmarshal(parameterLimitsRaw, &item.ParameterLimits)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}
	return out, nil
}

func (s *CatalogService) CreateAPIKey(ctx context.Context, in schemas.APIKeyCreate) (schemas.APIKey, error) {
	rawKey := in.Key
	if rawKey == nil || strings.TrimSpace(*rawKey) == "" {
		gen, err := generateAPIKey()
		if err != nil {
			return schemas.APIKey{}, fmt.Errorf("generate api key: %w", err)
		}
		rawKey = &gen
	}
	if in.ParameterLimits == nil {
		in.ParameterLimits = map[string]any{}
	}
	allowedModelsRaw, _ := json.Marshal(in.AllowedModels)
	allowedProvidersRaw, _ := json.Marshal(in.AllowedProviders)
	parameterLimitsRaw, _ := json.Marshal(in.ParameterLimits)

	var item schemas.APIKey
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO api_keys (key, name, is_active, allowed_models, allowed_providers, parameter_limits, created_at, updated_at)
		VALUES ($1,$2,true,$3,$4,$5,now(),now())
		RETURNING id, key, name, is_active, allowed_models, allowed_providers, parameter_limits, created_at, updated_at
	`, rawKey, in.Name, allowedModelsRaw, allowedProvidersRaw, parameterLimitsRaw).Scan(
		&item.ID, &item.Key, &item.Name, &item.IsActive,
		&allowedModelsRaw, &allowedProvidersRaw, &parameterLimitsRaw, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return schemas.APIKey{}, fmt.Errorf("create api key: %w", err)
	}
	_ = json.Unmarshal(allowedModelsRaw, &item.AllowedModels)
	_ = json.Unmarshal(allowedProvidersRaw, &item.AllowedProviders)
	_ = json.Unmarshal(parameterLimitsRaw, &item.ParameterLimits)
	return item, nil
}

func (s *CatalogService) GetAPIKey(ctx context.Context, id int64) (schemas.APIKey, error) {
	var (
		item                schemas.APIKey
		allowedModelsRaw    []byte
		allowedProvidersRaw []byte
		parameterLimitsRaw  []byte
	)
	if err := s.pool.QueryRow(ctx, `
		SELECT id, key, name, is_active, allowed_models, allowed_providers, parameter_limits, created_at, updated_at
		FROM api_keys WHERE id = $1
	`, id).Scan(
		&item.ID, &item.Key, &item.Name, &item.IsActive,
		&allowedModelsRaw, &allowedProvidersRaw, &parameterLimitsRaw, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return schemas.APIKey{}, ErrNotFound
		}
		return schemas.APIKey{}, fmt.Errorf("get api key: %w", err)
	}
	_ = json.Unmarshal(allowedModelsRaw, &item.AllowedModels)
	_ = json.Unmarshal(allowedProvidersRaw, &item.AllowedProviders)
	_ = json.Unmarshal(parameterLimitsRaw, &item.ParameterLimits)
	return item, nil
}

func (s *CatalogService) UpdateAPIKey(ctx context.Context, id int64, in schemas.APIKeyUpdate) (schemas.APIKey, error) {
	current, err := s.GetAPIKey(ctx, id)
	if err != nil {
		return schemas.APIKey{}, err
	}
	name := current.Name
	if in.Name != nil {
		name = in.Name
	}
	isActive := current.IsActive
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	allowedModels := current.AllowedModels
	if in.AllowedModels != nil {
		allowedModels = in.AllowedModels
	}
	allowedProviders := current.AllowedProviders
	if in.AllowedProviders != nil {
		allowedProviders = in.AllowedProviders
	}
	parameterLimits := current.ParameterLimits
	if in.ParameterLimits != nil {
		parameterLimits = in.ParameterLimits
	}
	if parameterLimits == nil {
		parameterLimits = map[string]any{}
	}
	allowedModelsRaw, _ := json.Marshal(allowedModels)
	allowedProvidersRaw, _ := json.Marshal(allowedProviders)
	parameterLimitsRaw, _ := json.Marshal(parameterLimits)

	var item schemas.APIKey
	if err := s.pool.QueryRow(ctx, `
		UPDATE api_keys
		SET name = $2, is_active = $3, allowed_models = $4, allowed_providers = $5, parameter_limits = $6, updated_at = now()
		WHERE id = $1
		RETURNING id, key, name, is_active, allowed_models, allowed_providers, parameter_limits, created_at, updated_at
	`, id, name, isActive, allowedModelsRaw, allowedProvidersRaw, parameterLimitsRaw).Scan(
		&item.ID, &item.Key, &item.Name, &item.IsActive,
		&allowedModelsRaw, &allowedProvidersRaw, &parameterLimitsRaw, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return schemas.APIKey{}, ErrNotFound
		}
		return schemas.APIKey{}, fmt.Errorf("update api key: %w", err)
	}
	_ = json.Unmarshal(allowedModelsRaw, &item.AllowedModels)
	_ = json.Unmarshal(allowedProvidersRaw, &item.AllowedProviders)
	_ = json.Unmarshal(parameterLimitsRaw, &item.ParameterLimits)
	return item, nil
}

func (s *CatalogService) DeleteAPIKey(ctx context.Context, id int64) error {
	tag, err := s.GetAPIKey(ctx, id)
	if err != nil {
		return err
	}
	if !tag.IsActive {
		return nil
	}
	_, err = s.pool.Exec(ctx, `UPDATE api_keys SET is_active = false, updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	return nil
}

func (s *CatalogService) ValidateAPIKey(ctx context.Context, key string) (schemas.APIKey, error) {
	var (
		item                schemas.APIKey
		allowedModelsRaw    []byte
		allowedProvidersRaw []byte
		parameterLimitsRaw  []byte
	)
	if err := s.pool.QueryRow(ctx, `
		SELECT id, key, name, is_active, allowed_models, allowed_providers, parameter_limits, created_at, updated_at
		FROM api_keys WHERE key = $1 AND is_active = true
	`, key).Scan(
		&item.ID, &item.Key, &item.Name, &item.IsActive,
		&allowedModelsRaw, &allowedProvidersRaw, &parameterLimitsRaw, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return schemas.APIKey{}, ErrNotFound
		}
		return schemas.APIKey{}, fmt.Errorf("validate api key: %w", err)
	}
	_ = json.Unmarshal(allowedModelsRaw, &item.AllowedModels)
	_ = json.Unmarshal(allowedProvidersRaw, &item.AllowedProviders)
	_ = json.Unmarshal(parameterLimitsRaw, &item.ParameterLimits)
	return item, nil
}

func generateAPIKey() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	out := make([]byte, len(buf))
	for i := range buf {
		out[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return "sk-" + string(out), nil
}

func (s *CatalogService) OpenAIChatCompletions(ctx context.Context, providerHint string, payload map[string]any) (map[string]any, error) {
	resp, err := s.invokeOpenAIChat(ctx, providerHint, payload, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	out := map[string]any{}
	if len(respBody) > 0 && json.Valid(respBody) {
		_ = json.Unmarshal(respBody, &out)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("upstream returned non-json payload")
	}
	return out, nil
}

func (s *CatalogService) OpenAIChatCompletionsStream(ctx context.Context, providerHint string, payload map[string]any) (*StreamResponse, error) {
	resp, err := s.invokeOpenAIChat(ctx, providerHint, payload, true)
	if err != nil {
		return nil, err
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "text/event-stream"
	}
	return &StreamResponse{
		Body:        resp.Body,
		ContentType: contentType,
	}, nil
}

func (s *CatalogService) invokeOpenAIChat(ctx context.Context, providerHint string, payload map[string]any, forceStream bool) (*http.Response, error) {
	requestModel, _ := payload["model"].(string)
	if strings.TrimSpace(requestModel) == "" {
		return nil, fmt.Errorf("model is required")
	}
	target, err := s.resolveChatTarget(ctx, providerHint, requestModel)
	if err != nil {
		return nil, err
	}

	body := map[string]any{}
	for k, v := range payload {
		body[k] = v
	}
	if target.RemoteIdentifier != nil && strings.TrimSpace(*target.RemoteIdentifier) != "" {
		body["model"] = *target.RemoteIdentifier
	} else {
		body["model"] = target.ModelName
	}
	if forceStream {
		body["stream"] = true
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	endpoint := normalizeChatCompletionsEndpoint(target.ProviderBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if target.ProviderAPIKey != nil && strings.TrimSpace(*target.ProviderAPIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(*target.ProviderAPIKey))
	}

	client := &http.Client{Timeout: 90 * time.Second}
	if forceStream {
		client.Timeout = 0
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoke upstream: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		out := map[string]any{}
		if len(respBody) > 0 && json.Valid(respBody) {
			_ = json.Unmarshal(respBody, &out)
		}
		detail := strings.TrimSpace(string(respBody))
		if len(out) > 0 {
			detail = fmt.Sprintf("%v", out)
		}
		if detail == "" {
			detail = http.StatusText(resp.StatusCode)
		}
		return nil, &UpstreamStatusError{StatusCode: resp.StatusCode, Detail: detail}
	}
	return resp, nil
}

type chatTarget struct {
	ProviderName     string
	ProviderType     string
	ProviderBaseURL  *string
	ProviderAPIKey   *string
	ModelName        string
	RemoteIdentifier *string
}

func (s *CatalogService) resolveChatTarget(ctx context.Context, providerHint string, requestedModel string) (chatTarget, error) {
	providerHint = strings.TrimSpace(providerHint)
	requestedModel = strings.TrimSpace(requestedModel)
	if providerHint != "" {
		modelName := requestedModel
		if strings.HasPrefix(modelName, providerHint+"/") {
			modelName = strings.TrimPrefix(modelName, providerHint+"/")
		}
		if target, ok, err := s.queryChatTarget(ctx, providerHint, modelName, ""); err != nil {
			return chatTarget{}, err
		} else if ok {
			return target, nil
		}
		if target, ok, err := s.queryChatTarget(ctx, providerHint, "", requestedModel); err != nil {
			return chatTarget{}, err
		} else if ok {
			return target, nil
		}
		return chatTarget{}, ErrNotFound
	}

	if parts := strings.SplitN(requestedModel, "/", 2); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		if target, ok, err := s.queryChatTarget(ctx, parts[0], parts[1], requestedModel); err != nil {
			return chatTarget{}, err
		} else if ok {
			return target, nil
		}
	}

	if target, ok, err := s.queryChatTargetByModelName(ctx, requestedModel); err != nil {
		return chatTarget{}, err
	} else if ok {
		return target, nil
	}
	if target, ok, err := s.queryChatTargetByRemoteIdentifier(ctx, requestedModel); err != nil {
		return chatTarget{}, err
	} else if ok {
		return target, nil
	}
	return chatTarget{}, ErrNotFound
}

func (s *CatalogService) queryChatTarget(ctx context.Context, providerName string, modelName string, remoteIdentifier string) (chatTarget, bool, error) {
	query := `
		SELECT p.name, p.type, p.base_url, p.api_key, m.name, m.remote_identifier
		FROM models m
		JOIN providers p ON p.id = m.provider_id
		WHERE p.is_active = true AND m.is_active = true AND p.name = $1
	`
	args := []any{providerName}
	if modelName != "" {
		query += ` AND m.name = $2`
		args = append(args, modelName)
	} else {
		query += ` AND m.remote_identifier = $2`
		args = append(args, remoteIdentifier)
	}
	query += ` ORDER BY m.id ASC LIMIT 1`

	var target chatTarget
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&target.ProviderName, &target.ProviderType, &target.ProviderBaseURL, &target.ProviderAPIKey, &target.ModelName, &target.RemoteIdentifier,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return chatTarget{}, false, nil
	}
	if err != nil {
		return chatTarget{}, false, fmt.Errorf("query chat target: %w", err)
	}
	return target, true, nil
}

func (s *CatalogService) queryChatTargetByModelName(ctx context.Context, modelName string) (chatTarget, bool, error) {
	var target chatTarget
	err := s.pool.QueryRow(ctx, `
		SELECT p.name, p.type, p.base_url, p.api_key, m.name, m.remote_identifier
		FROM models m
		JOIN providers p ON p.id = m.provider_id
		WHERE p.is_active = true AND m.is_active = true AND m.name = $1
		ORDER BY m.id ASC
		LIMIT 1
	`, modelName).Scan(
		&target.ProviderName, &target.ProviderType, &target.ProviderBaseURL, &target.ProviderAPIKey, &target.ModelName, &target.RemoteIdentifier,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return chatTarget{}, false, nil
	}
	if err != nil {
		return chatTarget{}, false, fmt.Errorf("query chat target by model name: %w", err)
	}
	return target, true, nil
}

func (s *CatalogService) queryChatTargetByRemoteIdentifier(ctx context.Context, modelRef string) (chatTarget, bool, error) {
	var target chatTarget
	err := s.pool.QueryRow(ctx, `
		SELECT p.name, p.type, p.base_url, p.api_key, m.name, m.remote_identifier
		FROM models m
		JOIN providers p ON p.id = m.provider_id
		WHERE p.is_active = true AND m.is_active = true AND m.remote_identifier = $1
		ORDER BY m.id ASC
		LIMIT 1
	`, modelRef).Scan(
		&target.ProviderName, &target.ProviderType, &target.ProviderBaseURL, &target.ProviderAPIKey, &target.ModelName, &target.RemoteIdentifier,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return chatTarget{}, false, nil
	}
	if err != nil {
		return chatTarget{}, false, fmt.Errorf("query chat target by remote identifier: %w", err)
	}
	return target, true, nil
}

func normalizeChatCompletionsEndpoint(baseURL *string) string {
	base := "https://api.openai.com/v1"
	if baseURL != nil && strings.TrimSpace(*baseURL) != "" {
		base = strings.TrimSpace(*baseURL)
	}
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}

func (s *CatalogService) GeminiGenerateContent(ctx context.Context, modelName string, payload map[string]any) (map[string]any, error) {
	contents, _ := payload["contents"].([]any)
	messages := make([]map[string]any, 0, len(contents))
	for _, item := range contents {
		contentMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := contentMap["role"].(string)
		if role == "" {
			role = "user"
		}
		parts, _ := contentMap["parts"].([]any)
		textParts := make([]string, 0, len(parts))
		for _, p := range parts {
			partMap, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := partMap["text"].(string); ok && strings.TrimSpace(text) != "" {
				textParts = append(textParts, text)
			}
		}
		if len(textParts) == 0 {
			continue
		}
		messages = append(messages, map[string]any{
			"role":    role,
			"content": strings.Join(textParts, "\n"),
		})
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("contents is required")
	}

	openAIPayload := map[string]any{
		"model":    modelName,
		"messages": messages,
	}
	if generationConfig, ok := payload["generationConfig"].(map[string]any); ok {
		params := map[string]any{}
		for _, key := range []string{"temperature", "topP", "topK", "maxOutputTokens"} {
			if v, exists := generationConfig[key]; exists {
				params[key] = v
			}
		}
		for k, v := range params {
			switch k {
			case "maxOutputTokens":
				openAIPayload["max_tokens"] = v
			case "topP":
				openAIPayload["top_p"] = v
			case "topK":
				openAIPayload["top_k"] = v
			default:
				openAIPayload[k] = v
			}
		}
	}

	openAIResp, err := s.OpenAIChatCompletions(ctx, "gemini", openAIPayload)
	if err != nil {
		return nil, err
	}

	text := extractFirstAssistantText(openAIResp)
	out := map[string]any{
		"candidates": []map[string]any{
			{
				"content": map[string]any{
					"role": "model",
					"parts": []map[string]any{
						{"text": text},
					},
				},
				"finishReason": "STOP",
				"index":        0,
			},
		},
		"modelVersion": modelName,
	}
	if usage, ok := openAIResp["usage"].(map[string]any); ok {
		out["usageMetadata"] = map[string]any{
			"promptTokenCount":     usage["prompt_tokens"],
			"candidatesTokenCount": usage["completion_tokens"],
			"totalTokenCount":      usage["total_tokens"],
		}
	}
	return out, nil
}

func (s *CatalogService) GeminiStreamGenerateContent(ctx context.Context, modelName string, payload map[string]any) (*StreamResponse, error) {
	contents, _ := payload["contents"].([]any)
	messages := make([]map[string]any, 0, len(contents))
	for _, item := range contents {
		contentMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := contentMap["role"].(string)
		if role == "" {
			role = "user"
		}
		parts, _ := contentMap["parts"].([]any)
		textParts := make([]string, 0, len(parts))
		for _, p := range parts {
			partMap, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := partMap["text"].(string); ok && strings.TrimSpace(text) != "" {
				textParts = append(textParts, text)
			}
		}
		if len(textParts) == 0 {
			continue
		}
		messages = append(messages, map[string]any{
			"role":    role,
			"content": strings.Join(textParts, "\n"),
		})
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("contents is required")
	}
	openAIPayload := map[string]any{
		"model":    modelName,
		"messages": messages,
		"stream":   true,
	}
	if generationConfig, ok := payload["generationConfig"].(map[string]any); ok {
		params := map[string]any{}
		for _, key := range []string{"temperature", "topP", "topK", "maxOutputTokens"} {
			if v, exists := generationConfig[key]; exists {
				params[key] = v
			}
		}
		for k, v := range params {
			switch k {
			case "maxOutputTokens":
				openAIPayload["max_tokens"] = v
			case "topP":
				openAIPayload["top_p"] = v
			case "topK":
				openAIPayload["top_k"] = v
			default:
				openAIPayload[k] = v
			}
		}
	}
	return s.OpenAIChatCompletionsStream(ctx, "gemini", openAIPayload)
}

func extractFirstAssistantText(openAIResp map[string]any) string {
	choices, ok := openAIResp["choices"].([]any)
	if !ok || len(choices) == 0 {
		return ""
	}
	choice0, ok := choices[0].(map[string]any)
	if !ok {
		return ""
	}
	msg, ok := choice0["message"].(map[string]any)
	if !ok {
		return ""
	}
	text, _ := msg["content"].(string)
	return text
}

func (s *CatalogService) ClaudeMessages(ctx context.Context, payload map[string]any) (map[string]any, error) {
	modelName, _ := payload["model"].(string)
	if strings.TrimSpace(modelName) == "" {
		return nil, fmt.Errorf("model is required")
	}
	messagesIn, _ := payload["messages"].([]any)
	messages := make([]map[string]any, 0, len(messagesIn)+1)
	if system, ok := payload["system"].(string); ok && strings.TrimSpace(system) != "" {
		messages = append(messages, map[string]any{"role": "system", "content": system})
	}
	for _, raw := range messagesIn {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role == "" {
			role = "user"
		}
		switch content := m["content"].(type) {
		case string:
			messages = append(messages, map[string]any{"role": role, "content": content})
		case []any:
			parts := make([]string, 0, len(content))
			for _, p := range content {
				pm, ok := p.(map[string]any)
				if !ok {
					continue
				}
				if pm["type"] == "text" {
					if t, ok := pm["text"].(string); ok && strings.TrimSpace(t) != "" {
						parts = append(parts, t)
					}
				}
			}
			if len(parts) > 0 {
				messages = append(messages, map[string]any{"role": role, "content": strings.Join(parts, "\n")})
			}
		}
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages is required")
	}

	openAIPayload := map[string]any{
		"model":    modelName,
		"messages": messages,
	}
	for _, key := range []string{"temperature", "top_p", "max_tokens"} {
		if v, ok := payload[key]; ok {
			openAIPayload[key] = v
		}
	}

	openAIResp, err := s.OpenAIChatCompletions(ctx, "claude", openAIPayload)
	if err != nil {
		return nil, err
	}
	text := extractFirstAssistantText(openAIResp)
	resp := map[string]any{
		"id":            openAIResp["id"],
		"type":          "message",
		"role":          "assistant",
		"model":         modelName,
		"content":       []map[string]any{{"type": "text", "text": text}},
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
	}
	if usage, ok := openAIResp["usage"].(map[string]any); ok {
		resp["usage"] = map[string]any{
			"input_tokens":  usage["prompt_tokens"],
			"output_tokens": usage["completion_tokens"],
		}
	}
	return resp, nil
}

func (s *CatalogService) ClaudeCountTokens(_ context.Context, payload map[string]any) (map[string]any, error) {
	var totalChars int
	if system, ok := payload["system"].(string); ok {
		totalChars += len([]rune(system))
	}
	if messages, ok := payload["messages"].([]any); ok {
		for _, raw := range messages {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			switch content := m["content"].(type) {
			case string:
				totalChars += len([]rune(content))
			case []any:
				for _, p := range content {
					pm, ok := p.(map[string]any)
					if !ok {
						continue
					}
					if t, ok := pm["text"].(string); ok {
						totalChars += len([]rune(t))
					}
				}
			}
		}
	}
	// Rough estimate used as temporary implementation before provider-native tokenizer support.
	inputTokens := totalChars / 4
	if inputTokens < 1 && totalChars > 0 {
		inputTokens = 1
	}
	return map[string]any{"input_tokens": inputTokens}, nil
}

func (s *CatalogService) ClaudeCreateMessageBatch(ctx context.Context, payload map[string]any) (map[string]any, error) {
	return s.invokeClaudeBatch(ctx, http.MethodPost, "/v1/messages/batches", payload)
}

func (s *CatalogService) ClaudeGetMessageBatch(ctx context.Context, batchID string) (map[string]any, error) {
	if strings.TrimSpace(batchID) == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	return s.invokeClaudeBatch(ctx, http.MethodGet, "/v1/messages/batches/"+batchID, nil)
}

func (s *CatalogService) ClaudeCancelMessageBatch(ctx context.Context, batchID string) (map[string]any, error) {
	if strings.TrimSpace(batchID) == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	return s.invokeClaudeBatch(ctx, http.MethodPost, "/v1/messages/batches/"+batchID+"/cancel", nil)
}

func (s *CatalogService) invokeClaudeBatch(ctx context.Context, method string, endpointPath string, payload map[string]any) (map[string]any, error) {
	provider, err := s.resolveClaudeProvider(ctx)
	if err != nil {
		return nil, err
	}
	apiKey := ""
	if provider.APIKey != nil {
		apiKey = strings.TrimSpace(*provider.APIKey)
	}
	if apiKey == "" {
		if raw, ok := provider.Settings["api_key"].(string); ok {
			apiKey = strings.TrimSpace(raw)
		}
	}
	if apiKey == "" {
		return nil, fmt.Errorf("claude api_key is required")
	}

	baseURL := "https://api.anthropic.com"
	if provider.BaseURL != nil && strings.TrimSpace(*provider.BaseURL) != "" {
		baseURL = strings.TrimSpace(*provider.BaseURL)
	} else if raw, ok := provider.Settings["base_url"].(string); ok && strings.TrimSpace(raw) != "" {
		baseURL = strings.TrimSpace(raw)
	}
	endpoint := strings.TrimRight(baseURL, "/") + endpointPath
	if raw, ok := provider.Settings["anthropic_base_url"].(string); ok && strings.TrimSpace(raw) != "" {
		endpoint = strings.TrimRight(strings.TrimSpace(raw), "/") + endpointPath
	}

	var reader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal claude batch payload: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, fmt.Errorf("build claude batch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	version := "2023-06-01"
	if raw, ok := provider.Settings["anthropic_version"].(string); ok && strings.TrimSpace(raw) != "" {
		version = strings.TrimSpace(raw)
	}
	req.Header.Set("anthropic-version", version)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoke claude batch endpoint: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(respBody) > 0 && json.Valid(respBody) {
		_ = json.Unmarshal(respBody, &out)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := strings.TrimSpace(string(respBody))
		if len(out) > 0 {
			detail = fmt.Sprintf("%v", out)
		}
		if detail == "" {
			detail = http.StatusText(resp.StatusCode)
		}
		return nil, &UpstreamStatusError{StatusCode: resp.StatusCode, Detail: detail}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("claude batch upstream returned non-json payload")
	}
	return out, nil
}

func (s *CatalogService) resolveClaudeProvider(ctx context.Context) (db.Provider, error) {
	var (
		p           db.Provider
		settingsRaw []byte
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, type, is_active, base_url, api_key, settings, created_at, updated_at
		FROM providers
		WHERE is_active = true AND name = 'claude'
		ORDER BY id ASC
		LIMIT 1
	`).Scan(&p.ID, &p.Name, &p.Type, &p.IsActive, &p.BaseURL, &p.APIKey, &settingsRaw, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `
			SELECT id, name, type, is_active, base_url, api_key, settings, created_at, updated_at
			FROM providers
			WHERE is_active = true AND type = $1
			ORDER BY id ASC
			LIMIT 1
		`, db.ProviderTypeClaude).Scan(&p.ID, &p.Name, &p.Type, &p.IsActive, &p.BaseURL, &p.APIKey, &settingsRaw, &p.CreatedAt, &p.UpdatedAt)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Provider{}, ErrNotFound
		}
		return db.Provider{}, fmt.Errorf("resolve claude provider: %w", err)
	}
	if len(settingsRaw) > 0 {
		_ = json.Unmarshal(settingsRaw, &p.Settings)
	}
	if p.Settings == nil {
		p.Settings = map[string]any{}
	}
	return p, nil
}

func (s *CatalogService) ExportMonitorDatabaseSQLite(ctx context.Context) ([]byte, error) {
	type monitorRow struct {
		ID                 int64
		ModelID            int64
		ProviderID         int64
		ModelName          string
		ProviderName       string
		StartedAt          *time.Time
		CompletedAt        *time.Time
		DurationMS         *float64
		Status             string
		ErrorMessage       *string
		RequestPrompt      *string
		ResponseText       *string
		ResponseTextLength *int64
		PromptTokens       *int64
		CompletionTokens   *int64
		TotalTokens        *int64
		Cost               *float64
		CreatedAt          *time.Time
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			id, model_id, provider_id, model_name, provider_name,
			started_at, completed_at, duration_ms, status, error_message,
			request_prompt, response_text, response_text_length,
			prompt_tokens, completion_tokens, total_tokens, cost, created_at
		FROM monitor_invocations
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query monitor invocations for sqlite export: %w", err)
	}
	defer rows.Close()

	items := make([]monitorRow, 0)
	for rows.Next() {
		var item monitorRow
		if err := rows.Scan(
			&item.ID, &item.ModelID, &item.ProviderID, &item.ModelName, &item.ProviderName,
			&item.StartedAt, &item.CompletedAt, &item.DurationMS, &item.Status, &item.ErrorMessage,
			&item.RequestPrompt, &item.ResponseText, &item.ResponseTextLength,
			&item.PromptTokens, &item.CompletionTokens, &item.TotalTokens, &item.Cost, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan monitor invocation for sqlite export: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monitor invocations for sqlite export: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "llm_datas_*.db")
	if err != nil {
		return nil, fmt.Errorf("create temp sqlite file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)

	sqliteDB, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return nil, fmt.Errorf("open temp sqlite file: %w", err)
	}
	defer sqliteDB.Close()

	if _, err := sqliteDB.Exec(`
		CREATE TABLE IF NOT EXISTS monitor_invocations (
			id INTEGER PRIMARY KEY,
			model_id INTEGER NOT NULL,
			provider_id INTEGER NOT NULL,
			model_name TEXT NOT NULL,
			provider_name TEXT NOT NULL,
			started_at TEXT NOT NULL,
			completed_at TEXT,
			duration_ms REAL,
			status TEXT NOT NULL,
			error_message TEXT,
			request_prompt TEXT,
			response_text TEXT,
			response_text_length INTEGER,
			prompt_tokens INTEGER,
			completion_tokens INTEGER,
			total_tokens INTEGER,
			cost REAL,
			created_at TEXT
		)
	`); err != nil {
		return nil, fmt.Errorf("create monitor_invocations sqlite table: %w", err)
	}

	stmt, err := sqliteDB.Prepare(`
		INSERT INTO monitor_invocations (
			id, model_id, provider_id, model_name, provider_name,
			started_at, completed_at, duration_ms, status, error_message,
			request_prompt, response_text, response_text_length,
			prompt_tokens, completion_tokens, total_tokens, cost, created_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare sqlite insert monitor_invocations: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		if _, err := stmt.Exec(
			item.ID,
			item.ModelID,
			item.ProviderID,
			item.ModelName,
			item.ProviderName,
			timePtrString(item.StartedAt),
			timePtrString(item.CompletedAt),
			floatPtr(item.DurationMS),
			item.Status,
			stringPtr(item.ErrorMessage),
			stringPtr(item.RequestPrompt),
			stringPtr(item.ResponseText),
			intPtr(item.ResponseTextLength),
			intPtr(item.PromptTokens),
			intPtr(item.CompletionTokens),
			intPtr(item.TotalTokens),
			floatPtr(item.Cost),
			timePtrString(item.CreatedAt),
		); err != nil {
			return nil, fmt.Errorf("insert sqlite monitor_invocations id=%d: %w", item.ID, err)
		}
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read sqlite export file: %w", err)
	}
	return data, nil
}

func (s *CatalogService) ListInvocations(ctx context.Context, limit int, offset int) ([]schemas.MonitorInvocation, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, model_id, provider_id, model_name, provider_name, started_at, completed_at, duration_ms,
		       status, error_message, request_prompt, response_text, response_text_length,
		       prompt_tokens, completion_tokens, total_tokens, cost, created_at
		FROM monitor_invocations
		ORDER BY id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list invocations query: %w", err)
	}
	defer rows.Close()

	out := make([]schemas.MonitorInvocation, 0)
	for rows.Next() {
		var item schemas.MonitorInvocation
		if err := rows.Scan(
			&item.ID, &item.ModelID, &item.ProviderID, &item.ModelName, &item.ProviderName, &item.StartedAt, &item.CompletedAt, &item.DurationMS,
			&item.Status, &item.ErrorMessage, &item.RequestPrompt, &item.ResponseText, &item.ResponseTextLength,
			&item.PromptTokens, &item.CompletionTokens, &item.TotalTokens, &item.Cost, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan invocation: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invocations: %w", err)
	}
	return out, nil
}

func (s *CatalogService) GetInvocation(ctx context.Context, id int64) (schemas.MonitorInvocation, error) {
	var item schemas.MonitorInvocation
	if err := s.pool.QueryRow(ctx, `
		SELECT id, model_id, provider_id, model_name, provider_name, started_at, completed_at, duration_ms,
		       status, error_message, request_prompt, response_text, response_text_length,
		       prompt_tokens, completion_tokens, total_tokens, cost, created_at
		FROM monitor_invocations
		WHERE id = $1
	`, id).Scan(
		&item.ID, &item.ModelID, &item.ProviderID, &item.ModelName, &item.ProviderName, &item.StartedAt, &item.CompletedAt, &item.DurationMS,
		&item.Status, &item.ErrorMessage, &item.RequestPrompt, &item.ResponseText, &item.ResponseTextLength,
		&item.PromptTokens, &item.CompletionTokens, &item.TotalTokens, &item.Cost, &item.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return schemas.MonitorInvocation{}, ErrNotFound
		}
		return schemas.MonitorInvocation{}, fmt.Errorf("get invocation: %w", err)
	}
	return item, nil
}

func (s *CatalogService) GetInvocationStatistics(ctx context.Context) (map[string]any, error) {
	var (
		total       int64
		successCnt  int64
		errorCnt    int64
		totalCost   *float64
		totalTokens *int64
	)
	if err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'success') AS success_count,
			COUNT(*) FILTER (WHERE status = 'error') AS error_count,
			COALESCE(SUM(cost), 0) AS total_cost,
			COALESCE(SUM(total_tokens), 0) AS total_tokens
		FROM monitor_invocations
	`).Scan(&total, &successCnt, &errorCnt, &totalCost, &totalTokens); err != nil {
		return nil, fmt.Errorf("get invocation statistics: %w", err)
	}
	resp := map[string]any{
		"total_invocations": total,
		"success_count":     successCnt,
		"error_count":       errorCnt,
		"total_cost":        0.0,
		"total_tokens":      int64(0),
	}
	if totalCost != nil {
		resp["total_cost"] = *totalCost
	}
	if totalTokens != nil {
		resp["total_tokens"] = *totalTokens
	}
	return resp, nil
}

func (s *CatalogService) ExportInvocationsCSV(ctx context.Context, limit int, offset int) ([]byte, error) {
	items, err := s.ListInvocations(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"id", "provider_name", "model_name", "status", "prompt_tokens", "completion_tokens", "total_tokens", "cost", "started_at",
	})
	for _, item := range items {
		record := []string{
			strconv.FormatInt(item.ID, 10),
			item.ProviderName,
			item.ModelName,
			item.Status,
			intPtrString(item.PromptTokens),
			intPtrString(item.CompletionTokens),
			intPtrString(item.TotalTokens),
			floatPtrString(item.Cost),
			timePtrString(item.StartedAt),
		}
		_ = w.Write(record)
	}
	w.Flush()
	return buf.Bytes(), nil
}

func (s *CatalogService) GetLatestPricing(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT model_name, provider_name,
		       AVG(COALESCE(cost,0)) AS avg_cost,
		       COUNT(*) AS sample_count
		FROM monitor_invocations
		WHERE completed_at IS NOT NULL
		GROUP BY model_name, provider_name
		ORDER BY model_name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query latest pricing: %w", err)
	}
	defer rows.Close()

	out := make([]map[string]any, 0)
	for rows.Next() {
		var (
			modelName   string
			provider    string
			avgCost     float64
			sampleCount int64
		)
		if err := rows.Scan(&modelName, &provider, &avgCost, &sampleCount); err != nil {
			return nil, fmt.Errorf("scan latest pricing: %w", err)
		}
		out = append(out, map[string]any{
			"model":        modelName,
			"provider":     provider,
			"avg_cost":     avgCost,
			"sample_count": sampleCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest pricing: %w", err)
	}
	return out, nil
}

func (s *CatalogService) GetPricingSuggestions(ctx context.Context) ([]map[string]any, error) {
	items, err := s.GetLatestPricing(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		ic, _ := items[i]["avg_cost"].(float64)
		jc, _ := items[j]["avg_cost"].(float64)
		return ic < jc
	})
	if len(items) > 10 {
		items = items[:10]
	}
	for i := range items {
		items[i]["reason"] = "lower observed average cost"
	}
	return items, nil
}

func intPtrString(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func intPtr(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func floatPtrString(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func floatPtr(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func stringPtr(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func timePtrString(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}
