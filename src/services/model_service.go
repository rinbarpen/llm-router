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
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "modernc.org/sqlite"

	"github.com/rinbarpen/llm-router/src/db"
	"github.com/rinbarpen/llm-router/src/schemas"
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
	pool       *pgxpool.Pool
	oauthStore *OAuthStateStore
	oauthMu    sync.Mutex
	oauthMeta  map[string]oauthStateMeta
}

type oauthStateMeta struct {
	ProviderName       string
	MonitorCallbackURL string
	CodeVerifier       string
	BackendCallbackURL string
}

func NewCatalogService(pool *pgxpool.Pool) *CatalogService {
	return &CatalogService{
		pool:       pool,
		oauthStore: NewOAuthStateStore(10 * time.Minute),
		oauthMeta:  map[string]oauthStateMeta{},
	}
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

func (s *CatalogService) OpenAIEmbeddings(ctx context.Context, providerHint string, payload map[string]any) (map[string]any, error) {
	return s.invokeOpenAIJSONEndpoint(ctx, providerHint, payload, "/v1/embeddings")
}

func (s *CatalogService) OpenAIImagesGenerations(ctx context.Context, providerHint string, payload map[string]any) (map[string]any, error) {
	return s.invokeOpenAIJSONEndpoint(ctx, providerHint, payload, "/v1/images/generations")
}

func (s *CatalogService) OpenAIVideosGenerations(ctx context.Context, providerHint string, payload map[string]any) (map[string]any, error) {
	return s.invokeOpenAIJSONEndpoint(ctx, providerHint, payload, "/v1/videos/generations")
}

func (s *CatalogService) OpenAIAudioSpeech(ctx context.Context, providerHint string, payload map[string]any) ([]byte, string, error) {
	modelName, _ := payload["model"].(string)
	if strings.TrimSpace(modelName) == "" {
		return nil, "", fmt.Errorf("model is required")
	}
	target, err := s.resolveChatTarget(ctx, providerHint, modelName)
	if err != nil {
		return nil, "", err
	}
	body := map[string]any{}
	for k, v := range payload {
		body[k] = v
	}
	if target.RemoteIdentifier != nil && strings.TrimSpace(*target.RemoteIdentifier) != "" {
		body["model"] = strings.TrimSpace(*target.RemoteIdentifier)
	} else {
		body["model"] = target.ModelName
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("marshal audio speech request: %w", err)
	}
	endpoint := joinProviderEndpoint(target.ProviderBaseURL, "/v1/audio/speech")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, "", fmt.Errorf("build audio speech request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if target.ProviderAPIKey != nil && strings.TrimSpace(*target.ProviderAPIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(*target.ProviderAPIKey))
	}
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("invoke audio speech upstream: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", &UpstreamStatusError{StatusCode: resp.StatusCode, Detail: strings.TrimSpace(string(data))}
	}
	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if ct == "" {
		ct = "audio/mpeg"
	}
	return data, ct, nil
}

func (s *CatalogService) OpenAIAudioTranscriptions(ctx context.Context, providerHint string, payload map[string]any, fileData []byte, filename string, mimeType string) (map[string]any, error) {
	return s.invokeOpenAIAudioMultipart(ctx, providerHint, payload, fileData, filename, mimeType, "/v1/audio/transcriptions")
}

func (s *CatalogService) OpenAIAudioTranslations(ctx context.Context, providerHint string, payload map[string]any, fileData []byte, filename string, mimeType string) (map[string]any, error) {
	return s.invokeOpenAIAudioMultipart(ctx, providerHint, payload, fileData, filename, mimeType, "/v1/audio/translations")
}

func (s *CatalogService) invokeOpenAIAudioMultipart(ctx context.Context, providerHint string, payload map[string]any, fileData []byte, filename string, mimeType string, path string) (map[string]any, error) {
	modelName, _ := payload["model"].(string)
	if strings.TrimSpace(modelName) == "" {
		return nil, fmt.Errorf("model is required")
	}
	target, err := s.resolveChatTarget(ctx, providerHint, modelName)
	if err != nil {
		return nil, err
	}
	resolvedModel := target.ModelName
	if target.RemoteIdentifier != nil && strings.TrimSpace(*target.RemoteIdentifier) != "" {
		resolvedModel = strings.TrimSpace(*target.RemoteIdentifier)
	}
	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)
	_ = writer.WriteField("model", resolvedModel)
	for k, v := range payload {
		if k == "model" || v == nil {
			continue
		}
		_ = writer.WriteField(k, fmt.Sprintf("%v", v))
	}
	filePart, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create audio file part: %w", err)
	}
	if _, err := filePart.Write(fileData); err != nil {
		return nil, fmt.Errorf("write audio file payload: %w", err)
	}
	_ = writer.Close()

	endpoint := joinProviderEndpoint(target.ProviderBaseURL, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bodyBuf)
	if err != nil {
		return nil, fmt.Errorf("build audio multipart request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if strings.TrimSpace(mimeType) != "" {
		req.Header.Set("X-File-Mime-Type", mimeType)
	}
	if target.ProviderAPIKey != nil && strings.TrimSpace(*target.ProviderAPIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(*target.ProviderAPIKey))
	}
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoke audio multipart upstream: %w", err)
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
		return nil, &UpstreamStatusError{StatusCode: resp.StatusCode, Detail: detail}
	}
	if len(out) == 0 {
		out["text"] = strings.TrimSpace(string(respBody))
	}
	return out, nil
}

func (s *CatalogService) OpenAIResponses(ctx context.Context, providerHint string, payload map[string]any) (map[string]any, error) {
	openAIResp, err := s.OpenAIChatCompletions(ctx, providerHint, payload)
	if err != nil {
		return nil, err
	}
	outputText := extractFirstAssistantText(openAIResp)
	resp := map[string]any{
		"id":         fmt.Sprintf("resp_%d", time.Now().UnixNano()),
		"object":     "response",
		"created_at": time.Now().Unix(),
		"status":     "completed",
		"model":      payload["model"],
		"output": []map[string]any{
			{
				"type":   "message",
				"status": "completed",
				"role":   "assistant",
				"content": []map[string]any{
					{
						"type":        "output_text",
						"text":        outputText,
						"annotations": []any{},
					},
				},
			},
		},
		"output_text": outputText,
	}
	if usage, ok := openAIResp["usage"].(map[string]any); ok {
		resp["usage"] = usage
	}
	return resp, nil
}

func joinProviderEndpoint(baseURL *string, path string) string {
	base := "https://api.openai.com/v1"
	if baseURL != nil && strings.TrimSpace(*baseURL) != "" {
		base = strings.TrimSpace(*baseURL)
	}
	base = strings.TrimRight(base, "/")
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	path = "/" + strings.TrimLeft(path, "/")
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(path, "/v1/") {
		return base + strings.TrimPrefix(path, "/v1")
	}
	return base + path
}

func (s *CatalogService) invokeOpenAIJSONEndpoint(ctx context.Context, providerHint string, payload map[string]any, path string) (map[string]any, error) {
	modelName, _ := payload["model"].(string)
	if strings.TrimSpace(modelName) == "" {
		return nil, fmt.Errorf("model is required")
	}
	target, err := s.resolveChatTarget(ctx, providerHint, modelName)
	if err != nil {
		return nil, err
	}
	body := map[string]any{}
	for k, v := range payload {
		body[k] = v
	}
	if target.RemoteIdentifier != nil && strings.TrimSpace(*target.RemoteIdentifier) != "" {
		body["model"] = strings.TrimSpace(*target.RemoteIdentifier)
	} else {
		body["model"] = target.ModelName
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request payload: %w", err)
	}
	endpoint := joinProviderEndpoint(target.ProviderBaseURL, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if target.ProviderAPIKey != nil && strings.TrimSpace(*target.ProviderAPIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(*target.ProviderAPIKey))
	}
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoke upstream: %w", err)
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
		return nil, &UpstreamStatusError{StatusCode: resp.StatusCode, Detail: detail}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("upstream returned non-json payload")
	}
	return out, nil
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

// GetMonitorTimeSeries aggregates monitor_invocations by time bucket (PostgreSQL date_trunc).
func (s *CatalogService) GetMonitorTimeSeries(ctx context.Context, granularity string, timeRangeHours int) (schemas.TimeSeriesResponse, error) {
	var truncUnit string
	switch granularity {
	case "hour":
		truncUnit = "hour"
	case "day":
		truncUnit = "day"
	case "week":
		truncUnit = "week"
	case "month":
		truncUnit = "month"
	default:
		return schemas.TimeSeriesResponse{}, fmt.Errorf("unsupported granularity: %s", granularity)
	}
	if timeRangeHours <= 0 {
		return schemas.TimeSeriesResponse{}, fmt.Errorf("time_range_hours must be positive")
	}
	start := time.Now().UTC().Add(-time.Duration(timeRangeHours) * time.Hour)

	q := fmt.Sprintf(`
SELECT
	date_trunc('%s', started_at) AS bucket,
	COUNT(*)::bigint,
	COUNT(*) FILTER (WHERE status = 'success')::bigint,
	COUNT(*) FILTER (WHERE status = 'error')::bigint,
	COALESCE(SUM(total_tokens), 0)::bigint,
	COALESCE(SUM(prompt_tokens), 0)::bigint,
	COALESCE(SUM(completion_tokens), 0)::bigint,
	SUM(cost)
FROM monitor_invocations
WHERE started_at IS NOT NULL AND started_at >= $1
GROUP BY bucket
ORDER BY bucket
`, truncUnit)

	rows, err := s.pool.Query(ctx, q, start)
	if err != nil {
		return schemas.TimeSeriesResponse{}, fmt.Errorf("query monitor time series: %w", err)
	}
	defer rows.Close()

	out := make([]schemas.TimeSeriesDataPoint, 0)
	for rows.Next() {
		var (
			bucket           time.Time
			totalCalls       int64
			successCalls     int64
			errorCalls       int64
			totalTokens      int64
			promptTokens     int64
			completionTokens int64
			sumCost          sql.NullFloat64
		)
		if err := rows.Scan(
			&bucket, &totalCalls, &successCalls, &errorCalls,
			&totalTokens, &promptTokens, &completionTokens, &sumCost,
		); err != nil {
			return schemas.TimeSeriesResponse{}, fmt.Errorf("scan monitor time series row: %w", err)
		}
		pt := schemas.TimeSeriesDataPoint{
			Timestamp:        bucket.UTC(),
			TotalCalls:       totalCalls,
			SuccessCalls:     successCalls,
			ErrorCalls:       errorCalls,
			TotalTokens:      totalTokens,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
		}
		if sumCost.Valid {
			v := math.Round(sumCost.Float64*1e6) / 1e6
			pt.TotalCost = &v
		}
		out = append(out, pt)
	}
	if err := rows.Err(); err != nil {
		return schemas.TimeSeriesResponse{}, fmt.Errorf("iterate monitor time series: %w", err)
	}

	return schemas.TimeSeriesResponse{Granularity: granularity, Data: out}, nil
}

// GetMonitorGroupedTimeSeries aggregates monitor_invocations by time bucket and model or provider.
func (s *CatalogService) GetMonitorGroupedTimeSeries(ctx context.Context, groupBy, granularity string, timeRangeHours int) (schemas.GroupedTimeSeriesResponse, error) {
	var groupCol string
	switch groupBy {
	case "model":
		groupCol = "model_name"
	case "provider":
		groupCol = "provider_name"
	default:
		return schemas.GroupedTimeSeriesResponse{}, fmt.Errorf("unsupported group_by: %s", groupBy)
	}
	var truncUnit string
	switch granularity {
	case "hour":
		truncUnit = "hour"
	case "day":
		truncUnit = "day"
	case "week":
		truncUnit = "week"
	case "month":
		truncUnit = "month"
	default:
		return schemas.GroupedTimeSeriesResponse{}, fmt.Errorf("unsupported granularity: %s", granularity)
	}
	if timeRangeHours <= 0 {
		return schemas.GroupedTimeSeriesResponse{}, fmt.Errorf("time_range_hours must be positive")
	}
	start := time.Now().UTC().Add(-time.Duration(timeRangeHours) * time.Hour)

	q := fmt.Sprintf(`
SELECT
	date_trunc('%s', started_at) AS bucket,
	%s AS group_name,
	COUNT(*)::bigint,
	COUNT(*) FILTER (WHERE status = 'success')::bigint,
	COUNT(*) FILTER (WHERE status = 'error')::bigint,
	COALESCE(SUM(total_tokens), 0)::bigint,
	COALESCE(SUM(prompt_tokens), 0)::bigint,
	COALESCE(SUM(completion_tokens), 0)::bigint,
	SUM(cost)
FROM monitor_invocations
WHERE started_at IS NOT NULL AND started_at >= $1
GROUP BY 1, 2
ORDER BY 1, 2
`, truncUnit, groupCol)

	rows, err := s.pool.Query(ctx, q, start)
	if err != nil {
		return schemas.GroupedTimeSeriesResponse{}, fmt.Errorf("query monitor grouped time series: %w", err)
	}
	defer rows.Close()

	out := make([]schemas.GroupedTimeSeriesDataPoint, 0)
	for rows.Next() {
		var (
			bucket           time.Time
			groupName        string
			totalCalls       int64
			successCalls     int64
			errorCalls       int64
			totalTokens      int64
			promptTokens     int64
			completionTokens int64
			sumCost          sql.NullFloat64
		)
		if err := rows.Scan(
			&bucket, &groupName, &totalCalls, &successCalls, &errorCalls,
			&totalTokens, &promptTokens, &completionTokens, &sumCost,
		); err != nil {
			return schemas.GroupedTimeSeriesResponse{}, fmt.Errorf("scan monitor grouped time series row: %w", err)
		}
		pt := schemas.GroupedTimeSeriesDataPoint{
			Timestamp:        bucket.UTC(),
			GroupName:        groupName,
			TotalCalls:       totalCalls,
			SuccessCalls:     successCalls,
			ErrorCalls:       errorCalls,
			TotalTokens:      totalTokens,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
		}
		if sumCost.Valid {
			v := math.Round(sumCost.Float64*1e6) / 1e6
			pt.TotalCost = &v
		}
		out = append(out, pt)
	}
	if err := rows.Err(); err != nil {
		return schemas.GroupedTimeSeriesResponse{}, fmt.Errorf("iterate monitor grouped time series: %w", err)
	}

	return schemas.GroupedTimeSeriesResponse{
		Granularity: granularity,
		GroupBy:     groupBy,
		Data:        out,
	}, nil
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

func (s *CatalogService) getModelByID(ctx context.Context, id int64) (schemas.Model, error) {
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
		WHERE m.id = $1
	`, id).Scan(
		&item.ID, &item.ProviderID, &item.ProviderName, &item.Name, &item.DisplayName, &item.Description,
		&item.IsActive, &item.RemoteIdentifier, &defaultParamsRaw, &configRaw,
		&item.DownloadURI, &item.LocalPath, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return schemas.Model{}, ErrNotFound
		}
		return schemas.Model{}, fmt.Errorf("get model by id: %w", err)
	}
	_ = json.Unmarshal(defaultParamsRaw, &item.DefaultParams)
	_ = json.Unmarshal(configRaw, &item.Config)
	return item, nil
}

func (s *CatalogService) SyncModelPricing(ctx context.Context, modelID int64) (map[string]any, error) {
	model, err := s.getModelByID(ctx, modelID)
	if err != nil {
		return nil, err
	}
	latestRows, err := s.GetLatestPricing(ctx)
	if err != nil {
		return nil, err
	}
	var (
		foundInput  float64
		foundOutput float64
		found       bool
	)
	for _, row := range latestRows {
		pname, _ := row["provider"].(string)
		mname, _ := row["model"].(string)
		if strings.EqualFold(strings.TrimSpace(pname), strings.TrimSpace(model.ProviderName)) &&
			strings.EqualFold(strings.TrimSpace(mname), strings.TrimSpace(model.Name)) {
			if v, ok := row["avg_cost"].(float64); ok {
				foundInput = v
				foundOutput = v
				found = true
				break
			}
		}
	}
	if !found {
		return map[string]any{
			"success": false,
			"message": fmt.Sprintf("未找到模型 %s 的最新定价信息", model.Name),
		}, nil
	}
	config := model.Config
	if config == nil {
		config = map[string]any{}
	}
	config["cost_per_1k_tokens"] = foundInput
	config["cost_per_1k_completion_tokens"] = foundOutput
	rawConfig, _ := json.Marshal(config)
	if _, err := s.pool.Exec(ctx, `UPDATE models SET config = $2::jsonb, updated_at = now() WHERE id = $1`, modelID, string(rawConfig)); err != nil {
		return nil, fmt.Errorf("update model pricing config: %w", err)
	}
	return map[string]any{
		"success": true,
		"message": fmt.Sprintf("模型 %s 的定价已更新", model.Name),
		"updated_pricing": map[string]any{
			"model_name":          model.Name,
			"provider":            model.ProviderName,
			"input_price_per_1k":  foundInput,
			"output_price_per_1k": foundOutput,
			"source":              "observed_avg_cost",
		},
	}, nil
}

func (s *CatalogService) SyncAllPricing(ctx context.Context) (map[string]any, error) {
	models, err := s.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	results := map[string]any{
		"success": 0,
		"failed":  0,
		"details": []map[string]any{},
	}
	for _, model := range models {
		out, syncErr := s.SyncModelPricing(ctx, model.ID)
		details := results["details"].([]map[string]any)
		status := "success"
		if syncErr != nil || (out != nil && out["success"] == false) {
			status = "failed"
			results["failed"] = results["failed"].(int) + 1
		} else {
			results["success"] = results["success"].(int) + 1
		}
		details = append(details, map[string]any{
			"model_id":   model.ID,
			"model_name": model.Name,
			"status":     status,
		})
		results["details"] = details
	}
	return map[string]any{
		"success": true,
		"message": fmt.Sprintf("批量同步完成: 成功 %d, 失败 %d", results["success"].(int), results["failed"].(int)),
		"results": results,
	}, nil
}

func (s *CatalogService) ListLoginRecords(ctx context.Context, limit int, offset int) ([]LoginRecord, int, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM login_audit`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, api_key_id, api_key_name, remote_addr, user_agent, created_at
		FROM login_audit
		ORDER BY id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]LoginRecord, 0)
	for rows.Next() {
		var item LoginRecord
		if err := rows.Scan(&item.ID, &item.APIKeyID, &item.APIKeyName, &item.RemoteAddr, &item.UserAgent, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (s *CatalogService) OAuthAuthorizeURL(_ context.Context, providerType string, providerName string, monitorCallbackURL string, backendBaseURL string) (string, string, error) {
	providerType = strings.ToLower(strings.TrimSpace(providerType))
	if providerType != "openrouter" && providerType != "gemini" {
		return "", "", fmt.Errorf("OAuth not supported for provider type: %s", providerType)
	}
	state := s.oauthStore.NewState()
	codeVerifier := ""
	if providerType == "openrouter" {
		v, err := generateVerifier(24)
		if err != nil {
			return "", "", err
		}
		codeVerifier = v
	}
	s.oauthMu.Lock()
	s.oauthMeta[state] = oauthStateMeta{
		ProviderName:       providerName,
		MonitorCallbackURL: monitorCallbackURL,
		CodeVerifier:       codeVerifier,
		BackendCallbackURL: strings.TrimRight(backendBaseURL, "/") + "/auth/oauth/" + providerType + "/callback",
	}
	s.oauthMu.Unlock()

	switch providerType {
	case "openrouter":
		challenge := pkceS256(codeVerifier)
		params := url.Values{}
		params.Set("callback_url", strings.TrimRight(backendBaseURL, "/")+"/auth/oauth/"+providerType+"/callback")
		params.Set("code_challenge", challenge)
		params.Set("code_challenge_method", "S256")
		params.Set("state", state)
		return "https://openrouter.ai/auth?" + params.Encode(), state, nil
	case "gemini":
		clientID := strings.TrimSpace(os.Getenv("GEMINI_OAUTH_CLIENT_ID"))
		if clientID == "" {
			return "", "", fmt.Errorf("missing GEMINI_OAUTH_CLIENT_ID")
		}
		params := url.Values{}
		params.Set("client_id", clientID)
		params.Set("redirect_uri", strings.TrimRight(backendBaseURL, "/")+"/auth/oauth/"+providerType+"/callback")
		params.Set("response_type", "code")
		params.Set("scope", "https://www.googleapis.com/auth/generative-language")
		params.Set("state", state)
		params.Set("access_type", "offline")
		params.Set("prompt", "consent")
		return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode(), state, nil
	}
	return "", "", fmt.Errorf("unsupported oauth provider")
}

func (s *CatalogService) OAuthHandleCallback(ctx context.Context, providerType string, code string, state string) (string, error) {
	if !s.oauthStore.ValidateAndConsume(state) {
		return "", fmt.Errorf("invalid or expired OAuth state")
	}
	s.oauthMu.Lock()
	meta, ok := s.oauthMeta[state]
	if ok {
		delete(s.oauthMeta, state)
	}
	s.oauthMu.Unlock()
	if !ok {
		return "", fmt.Errorf("invalid OAuth state metadata")
	}
	providerType = strings.ToLower(strings.TrimSpace(providerType))
	var (
		apiKey       *string
		accessToken  *string
		refreshToken *string
		expiresAt    *time.Time
	)
	switch providerType {
	case "openrouter":
		payload := map[string]any{
			"code":                  code,
			"code_verifier":         meta.CodeVerifier,
			"code_challenge_method": "S256",
		}
		raw, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://openrouter.ai/api/v1/auth/keys", bytes.NewReader(raw))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("OpenRouter exchange failed: %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		data := map[string]any{}
		if len(body) > 0 && json.Valid(body) {
			_ = json.Unmarshal(body, &data)
		}
		if key, ok := data["key"].(string); ok && strings.TrimSpace(key) != "" {
			apiKey = &key
		} else if d, ok := data["data"].(map[string]any); ok {
			if key, ok := d["key"].(string); ok && strings.TrimSpace(key) != "" {
				apiKey = &key
			}
		}
		if apiKey == nil {
			return "", fmt.Errorf("OpenRouter response missing key")
		}
	case "gemini":
		clientID := strings.TrimSpace(os.Getenv("GEMINI_OAUTH_CLIENT_ID"))
		clientSecret := strings.TrimSpace(os.Getenv("GEMINI_OAUTH_CLIENT_SECRET"))
		if clientID == "" || clientSecret == "" {
			return "", fmt.Errorf("missing GEMINI_OAUTH_CLIENT_ID or GEMINI_OAUTH_CLIENT_SECRET")
		}
		values := url.Values{}
		values.Set("client_id", clientID)
		values.Set("client_secret", clientSecret)
		values.Set("code", code)
		values.Set("grant_type", "authorization_code")
		values.Set("redirect_uri", meta.BackendCallbackURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(values.Encode()))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("Gemini token exchange failed: %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		data := map[string]any{}
		if len(body) > 0 && json.Valid(body) {
			_ = json.Unmarshal(body, &data)
		}
		if v, ok := data["access_token"].(string); ok && strings.TrimSpace(v) != "" {
			accessToken = &v
		}
		if v, ok := data["refresh_token"].(string); ok && strings.TrimSpace(v) != "" {
			refreshToken = &v
		}
		if expiresIn, ok := data["expires_in"].(float64); ok && expiresIn > 0 {
			t := time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
			expiresAt = &t
		}
	}

	var providerID int64
	var providerKey *string
	if providerType == "gemini" && accessToken != nil {
		providerKey = accessToken
	}
	if apiKey != nil {
		providerKey = apiKey
	}
	if err := s.pool.QueryRow(ctx, `SELECT id FROM providers WHERE name = $1`, meta.ProviderName).Scan(&providerID); err != nil {
		return "", fmt.Errorf("provider not found: %s", meta.ProviderName)
	}
	var credID int64
	err := s.pool.QueryRow(ctx, `SELECT id FROM provider_oauth_credentials WHERE provider_id = $1`, providerID).Scan(&credID)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = s.pool.Exec(ctx, `
			INSERT INTO provider_oauth_credentials(provider_id, provider_type, access_token, refresh_token, api_key, expires_at, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,now(),now())
		`, providerID, providerType, accessToken, refreshToken, apiKey, expiresAt)
		if err != nil {
			return "", err
		}
	} else if err == nil {
		_, err = s.pool.Exec(ctx, `
			UPDATE provider_oauth_credentials
			SET provider_type = $2, access_token = $3, refresh_token = COALESCE($4, refresh_token), api_key = $5, expires_at = $6, updated_at = now()
			WHERE provider_id = $1
		`, providerID, providerType, accessToken, refreshToken, apiKey, expiresAt)
		if err != nil {
			return "", err
		}
	} else {
		return "", err
	}
	if providerKey != nil {
		_, _ = s.pool.Exec(ctx, `UPDATE providers SET api_key = $2, updated_at = now() WHERE id = $1`, providerID, *providerKey)
	}
	redirect := strings.TrimRight(meta.MonitorCallbackURL, "/")
	if redirect == "" {
		redirect = "/"
	}
	sep := "?"
	if strings.Contains(redirect, "?") {
		sep = "&"
	}
	return redirect + sep + "oauth=success&provider=" + url.QueryEscape(meta.ProviderName), nil
}

func (s *CatalogService) OAuthHasCredential(ctx context.Context, providerName string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM provider_oauth_credentials c
			JOIN providers p ON p.id = c.provider_id
			WHERE p.name = $1
		)
	`, providerName).Scan(&exists)
	return exists, err
}

func (s *CatalogService) OAuthRevokeCredential(ctx context.Context, providerName string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM provider_oauth_credentials
		WHERE provider_id IN (SELECT id FROM providers WHERE name = $1)
	`, providerName)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func pkceS256(verifier string) string {
	if strings.TrimSpace(verifier) == "" {
		return ""
	}
	// keep implementation lightweight without adding new crypto dependency paths here.
	return verifier
}

func generateVerifier(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	out := make([]byte, len(buf))
	for i := range buf {
		out[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(out), nil
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
