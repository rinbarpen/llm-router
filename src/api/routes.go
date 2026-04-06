package api

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rinbarpen/llm-router/src/config"
	"github.com/rinbarpen/llm-router/src/schemas"
	"github.com/rinbarpen/llm-router/src/services"
)

const streamOpenRetryAttempts = 2

var (
	streamOpenRetryBackoff = 120 * time.Millisecond
	streamIdleTimeout      = 45 * time.Second
)

type CatalogService interface {
	ListProviders(ctx context.Context) ([]schemas.Provider, error)
	GetProviderByName(ctx context.Context, name string) (schemas.Provider, error)
	CreateProvider(ctx context.Context, in schemas.ProviderCreate) (schemas.Provider, error)
	UpdateProvider(ctx context.Context, name string, in schemas.ProviderUpdate) (schemas.Provider, error)
	ListModels(ctx context.Context) ([]schemas.Model, error)
	CreateModel(ctx context.Context, in schemas.ModelCreate) (schemas.Model, error)
	ListModelsByProvider(ctx context.Context, providerName string) ([]schemas.Model, error)
	GetModelByProviderAndName(ctx context.Context, providerName string, modelName string) (schemas.Model, error)
	UpdateModel(ctx context.Context, providerName string, modelName string, in schemas.ModelUpdate) (schemas.Model, error)
	ListAPIKeys(ctx context.Context, includeInactive bool) ([]schemas.APIKey, error)
	CreateAPIKey(ctx context.Context, in schemas.APIKeyCreate) (schemas.APIKey, error)
	GetAPIKey(ctx context.Context, id int64) (schemas.APIKey, error)
	UpdateAPIKey(ctx context.Context, id int64, in schemas.APIKeyUpdate) (schemas.APIKey, error)
	DeleteAPIKey(ctx context.Context, id int64) error
	ValidateAPIKey(ctx context.Context, key string) (schemas.APIKey, error)
	OpenAIChatCompletions(ctx context.Context, providerHint string, payload map[string]any) (map[string]any, error)
	OpenAIChatCompletionsStream(ctx context.Context, providerHint string, payload map[string]any) (*services.StreamResponse, error)
	OpenAIEmbeddings(ctx context.Context, providerHint string, payload map[string]any) (map[string]any, error)
	OpenAIResponses(ctx context.Context, providerHint string, payload map[string]any) (map[string]any, error)
	OpenAIAudioSpeech(ctx context.Context, providerHint string, payload map[string]any) ([]byte, string, error)
	OpenAIAudioTranscriptions(ctx context.Context, providerHint string, payload map[string]any, fileData []byte, filename string, mimeType string) (map[string]any, error)
	OpenAIAudioTranslations(ctx context.Context, providerHint string, payload map[string]any, fileData []byte, filename string, mimeType string) (map[string]any, error)
	OpenAIImagesGenerations(ctx context.Context, providerHint string, payload map[string]any) (map[string]any, error)
	OpenAIVideosGenerations(ctx context.Context, providerHint string, payload map[string]any) (map[string]any, error)
	GeminiGenerateContent(ctx context.Context, modelName string, payload map[string]any) (map[string]any, error)
	GeminiStreamGenerateContent(ctx context.Context, modelName string, payload map[string]any) (*services.StreamResponse, error)
	ClaudeMessages(ctx context.Context, payload map[string]any) (map[string]any, error)
	ClaudeCountTokens(ctx context.Context, payload map[string]any) (map[string]any, error)
	ClaudeCreateMessageBatch(ctx context.Context, payload map[string]any) (map[string]any, error)
	ClaudeGetMessageBatch(ctx context.Context, batchID string) (map[string]any, error)
	ClaudeCancelMessageBatch(ctx context.Context, batchID string) (map[string]any, error)
	ListInvocations(ctx context.Context, limit int, offset int) ([]schemas.MonitorInvocation, error)
	GetInvocation(ctx context.Context, id int64) (schemas.MonitorInvocation, error)
	GetInvocationStatistics(ctx context.Context) (map[string]any, error)
	GetMonitorTimeSeries(ctx context.Context, granularity string, timeRangeHours int) (schemas.TimeSeriesResponse, error)
	GetMonitorGroupedTimeSeries(ctx context.Context, groupBy, granularity string, timeRangeHours int) (schemas.GroupedTimeSeriesResponse, error)
	ExportInvocationsCSV(ctx context.Context, limit int, offset int) ([]byte, error)
	ExportMonitorDatabaseSQLite(ctx context.Context) ([]byte, error)
	GetLatestPricing(ctx context.Context) ([]map[string]any, error)
	GetPricingSuggestions(ctx context.Context) ([]map[string]any, error)
	SyncModelPricing(ctx context.Context, modelID int64) (map[string]any, error)
	SyncAllPricing(ctx context.Context) (map[string]any, error)
	ListLoginRecords(ctx context.Context, limit int, offset int) ([]services.LoginRecord, int, error)
	SyncRouterTOML(ctx context.Context, configPath string) error
	OAuthAuthorizeURL(ctx context.Context, providerType string, providerName string, monitorCallbackURL string, backendBaseURL string, accountName string, setDefault bool) (string, string, error)
	OAuthHandleCallback(ctx context.Context, providerType string, code string, state string) (string, error)
	OAuthHasCredential(ctx context.Context, providerName string) (bool, error)
	OAuthRevokeCredential(ctx context.Context, providerName string) (bool, error)
	ListOAuthAccounts(ctx context.Context, providerName string) ([]schemas.OAuthAccount, error)
	UpdateOAuthAccount(ctx context.Context, providerName string, accountID int64, in schemas.OAuthAccountUpdate) (schemas.OAuthAccount, error)
	SetDefaultOAuthAccount(ctx context.Context, providerName string, accountID int64) (schemas.OAuthAccount, error)
	RevokeOAuthAccount(ctx context.Context, providerName string, accountID int64) (bool, error)
	RunSelfCheck(ctx context.Context) (map[string]any, error)
}

type RouterOptions struct {
	RequireAuth         bool
	AllowLocalNoAuth    bool
	ModelConfigHintPath string
}

func NewRouter(catalog ...CatalogService) http.Handler {
	var svc CatalogService
	if len(catalog) > 0 {
		svc = catalog[0]
	}
	return NewRouterWithOptions(svc, RouterOptions{
		RequireAuth:      false,
		AllowLocalNoAuth: true,
	})
}

func NewRouterWithOptions(svc CatalogService, opts RouterOptions) http.Handler {
	sessionStore := NewMemorySessionStore()
	r := chi.NewRouter()
	registerCoreRoutes(r, svc, sessionStore, opts)
	r.Mount("/api", apiCompatSubrouter(svc, sessionStore, opts))
	r.NotFound(notImplemented)
	r.MethodNotAllowed(notImplemented)
	return r
}

func apiCompatSubrouter(svc CatalogService, sessionStore SessionStore, opts RouterOptions) http.Handler {
	r := chi.NewRouter()
	registerCoreRoutes(r, svc, sessionStore, opts)
	r.NotFound(notImplemented)
	r.MethodNotAllowed(notImplemented)
	return r
}

func registerCoreRoutes(r chi.Router, svc CatalogService, sessions SessionStore, opts RouterOptions) {
	if opts.RequireAuth {
		r.Use(authMiddleware(svc, sessions, opts))
	}
	r.Get("/health", Health)
	r.Get("/health/detail", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		out, err := svc.RunSelfCheck(req.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "self check failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/providers", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providers, err := svc.ListProviders(req.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "list providers failed")
			return
		}
		writeJSON(w, http.StatusOK, providers)
	})
	r.Post("/providers", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		var payload schemas.ProviderCreate
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if payload.Name == "" || payload.Type == "" {
			writeJSONError(w, http.StatusBadRequest, "name and type are required")
			return
		}
		provider, err := svc.CreateProvider(req.Context(), payload)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "create provider failed")
			return
		}
		writeJSON(w, http.StatusCreated, provider)
	})
	r.Patch("/providers/{provider_name}", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerName := chi.URLParam(req, "provider_name")
		if providerName == "" {
			writeJSONError(w, http.StatusBadRequest, "provider_name is required")
			return
		}
		var payload schemas.ProviderUpdate
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		item, err := svc.UpdateProvider(req.Context(), providerName, payload)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "provider not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "update provider failed")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	r.Get("/models", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		models, err := svc.ListModels(req.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "list models failed")
			return
		}
		writeJSON(w, http.StatusOK, models)
	})
	r.Get("/models/{provider_name}", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerName := chi.URLParam(req, "provider_name")
		if providerName == "" {
			writeJSONError(w, http.StatusBadRequest, "provider_name is required")
			return
		}
		models, err := svc.ListModelsByProvider(req.Context(), providerName)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "list models by provider failed")
			return
		}
		writeJSON(w, http.StatusOK, models)
	})
	r.Post("/models", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		var payload schemas.ModelCreate
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if payload.ProviderName == "" || payload.Name == "" {
			writeJSONError(w, http.StatusBadRequest, "provider_name and name are required")
			return
		}
		model, err := svc.CreateModel(req.Context(), payload)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "provider not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "create model failed")
			return
		}
		writeJSON(w, http.StatusCreated, model)
	})
	r.Get("/models/{provider_name}/{model_name}", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerName := chi.URLParam(req, "provider_name")
		modelName := chi.URLParam(req, "model_name")
		if providerName == "" || modelName == "" {
			writeJSONError(w, http.StatusBadRequest, "provider_name and model_name are required")
			return
		}
		item, err := svc.GetModelByProviderAndName(req.Context(), providerName, modelName)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "model not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "get model failed")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	r.Patch("/models/{provider_name}/{model_name}", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerName := chi.URLParam(req, "provider_name")
		modelName := chi.URLParam(req, "model_name")
		if providerName == "" || modelName == "" {
			writeJSONError(w, http.StatusBadRequest, "provider_name and model_name are required")
			return
		}
		var payload schemas.ModelUpdate
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		item, err := svc.UpdateModel(req.Context(), providerName, modelName, payload)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "model not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "update model failed")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	r.Patch("/models/{provider_name}/{model_name}/metadata-override", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerName := chi.URLParam(req, "provider_name")
		modelName := chi.URLParam(req, "model_name")
		if providerName == "" || modelName == "" {
			writeJSONError(w, http.StatusBadRequest, "provider_name and model_name are required")
			return
		}
		payload, err := readJSONBody(req, false)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		current, err := svc.GetModelByProviderAndName(req.Context(), providerName, modelName)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "model not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "get model failed")
			return
		}
		configMap := map[string]any{}
		for k, v := range current.Config {
			configMap[k] = v
		}
		configMap["metadata_override"] = payload
		out, err := svc.UpdateModel(req.Context(), providerName, modelName, schemas.ModelUpdate{
			Config: configMap,
		})
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "update metadata override failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/v1/models", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		models, err := svc.ListModels(req.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "list models failed")
			return
		}
		out := schemas.OpenAIModelsResponse{
			Object: "list",
			Data:   make([]schemas.OpenAIModelObject, 0, len(models)),
		}
		now := time.Now().Unix()
		for _, model := range models {
			out.Data = append(out.Data, schemas.OpenAIModelObject{
				ID:      model.Name,
				Object:  "model",
				Created: now,
				OwnedBy: model.ProviderName,
			})
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/providers/{provider_name}/supported-models", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerName := chi.URLParam(req, "provider_name")
		if providerName == "" {
			writeJSONError(w, http.StatusBadRequest, "provider_name is required")
			return
		}
		models, err := svc.ListModelsByProvider(req.Context(), providerName)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "list models by provider failed")
			return
		}
		names := make([]string, 0, len(models))
		for _, model := range models {
			names = append(names, model.Name)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"provider_name": providerName,
			"models":        names,
		})
	})
	r.Post("/providers/{provider_name}/catalog-models/sync", func(w http.ResponseWriter, req *http.Request) {
		ps, ok := svc.(interface {
			SyncProviderModelCatalog(ctx context.Context, providerName string) (map[string]any, error)
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "provider catalog sync not supported")
			return
		}
		providerName := chi.URLParam(req, "provider_name")
		if strings.TrimSpace(providerName) == "" {
			writeJSONError(w, http.StatusBadRequest, "provider_name is required")
			return
		}
		out, err := ps.SyncProviderModelCatalog(req.Context(), providerName)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/providers/{provider_name}/catalog-models", func(w http.ResponseWriter, req *http.Request) {
		ps, ok := svc.(interface {
			ListProviderModelCatalog(ctx context.Context, providerName string) ([]map[string]any, error)
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "provider catalog list not supported")
			return
		}
		providerName := chi.URLParam(req, "provider_name")
		out, err := ps.ListProviderModelCatalog(req.Context(), providerName)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "list provider model catalog failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"provider_name": providerName,
			"models":        out,
		})
	})
	r.Get("/providers/{provider_name}/model-reconciliation", func(w http.ResponseWriter, req *http.Request) {
		ps, ok := svc.(interface {
			ReconcileProviderModels(ctx context.Context, providerName string) (map[string]any, error)
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "provider model reconciliation not supported")
			return
		}
		providerName := chi.URLParam(req, "provider_name")
		out, err := ps.ReconcileProviderModels(req.Context(), providerName)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "reconcile provider models failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/api-keys", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		var payload schemas.APIKeyCreate
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		item, err := svc.CreateAPIKey(req.Context(), payload)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "create api key failed")
			return
		}
		writeJSON(w, http.StatusCreated, item)
	})
	r.Get("/api-keys", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		includeInactive := req.URL.Query().Get("include_inactive") == "true"
		items, err := svc.ListAPIKeys(req.Context(), includeInactive)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "list api keys failed")
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
	r.Get("/api-keys/{id}", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		id, err := parseIDParam(req, "id")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid id")
			return
		}
		item, err := svc.GetAPIKey(req.Context(), id)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "api key not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "get api key failed")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	r.Patch("/api-keys/{id}", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		id, err := parseIDParam(req, "id")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid id")
			return
		}
		var payload schemas.APIKeyUpdate
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		item, err := svc.UpdateAPIKey(req.Context(), id, payload)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "api key not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "update api key failed")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	r.Delete("/api-keys/{id}", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		id, err := parseIDParam(req, "id")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid id")
			return
		}
		err = svc.DeleteAPIKey(req.Context(), id)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "api key not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "delete api key failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r.Get("/api-key-policy-templates", func(w http.ResponseWriter, req *http.Request) {
		ts, ok := svc.(interface {
			ListAPIKeyPolicyTemplates(ctx context.Context, teamTag, envTag string) ([]map[string]any, error)
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "policy template not supported")
			return
		}
		teamTag := strings.TrimSpace(req.URL.Query().Get("team_tag"))
		envTag := strings.TrimSpace(req.URL.Query().Get("env_tag"))
		out, err := ts.ListAPIKeyPolicyTemplates(req.Context(), teamTag, envTag)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "list policy templates failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/api-key-policy-templates", func(w http.ResponseWriter, req *http.Request) {
		ts, ok := svc.(interface {
			CreateAPIKeyPolicyTemplate(ctx context.Context, name string, teamTag *string, envTag *string, policy map[string]any) (map[string]any, error)
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "policy template not supported")
			return
		}
		payload, err := readJSONBody(req, false)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		name, _ := payload["name"].(string)
		if strings.TrimSpace(name) == "" {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}
		var (
			teamTag *string
			envTag  *string
		)
		if v, ok := payload["team_tag"].(string); ok && strings.TrimSpace(v) != "" {
			val := strings.TrimSpace(v)
			teamTag = &val
		}
		if v, ok := payload["env_tag"].(string); ok && strings.TrimSpace(v) != "" {
			val := strings.TrimSpace(v)
			envTag = &val
		}
		policy, _ := payload["policy"].(map[string]any)
		out, err := ts.CreateAPIKeyPolicyTemplate(req.Context(), name, teamTag, envTag, policy)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "create policy template failed")
			return
		}
		writeJSON(w, http.StatusCreated, out)
	})
	r.Patch("/api-key-policy-templates/{id}", func(w http.ResponseWriter, req *http.Request) {
		ts, ok := svc.(interface {
			UpdateAPIKeyPolicyTemplate(ctx context.Context, id int64, name string, teamTag *string, envTag *string, policy map[string]any) (map[string]any, error)
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "policy template not supported")
			return
		}
		id, err := parseIDParam(req, "id")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid id")
			return
		}
		payload, err := readJSONBody(req, false)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		name, _ := payload["name"].(string)
		var (
			teamTag *string
			envTag  *string
		)
		if v, ok := payload["team_tag"].(string); ok && strings.TrimSpace(v) != "" {
			val := strings.TrimSpace(v)
			teamTag = &val
		}
		if v, ok := payload["env_tag"].(string); ok && strings.TrimSpace(v) != "" {
			val := strings.TrimSpace(v)
			envTag = &val
		}
		policy, _ := payload["policy"].(map[string]any)
		out, err := ts.UpdateAPIKeyPolicyTemplate(req.Context(), id, name, teamTag, envTag, policy)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "update policy template failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Delete("/api-key-policy-templates/{id}", func(w http.ResponseWriter, req *http.Request) {
		ts, ok := svc.(interface {
			DeleteAPIKeyPolicyTemplate(ctx context.Context, id int64) error
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "policy template not supported")
			return
		}
		id, err := parseIDParam(req, "id")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid id")
			return
		}
		if err := ts.DeleteAPIKeyPolicyTemplate(req.Context(), id); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "delete policy template failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r.Post("/api-keys/batch-apply-policy", func(w http.ResponseWriter, req *http.Request) {
		ts, ok := svc.(interface {
			ApplyAPIKeyPolicyTemplate(ctx context.Context, templateID int64, apiKeyIDs []int64) (map[string]any, error)
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "batch apply policy not supported")
			return
		}
		payload, err := readJSONBody(req, false)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		templateID, ok := toInt64(payload["template_id"])
		if !ok || templateID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "template_id is required")
			return
		}
		rawIDs, _ := payload["api_key_ids"].([]any)
		ids := make([]int64, 0, len(rawIDs))
		for _, raw := range rawIDs {
			if id, ok := toInt64(raw); ok && id > 0 {
				ids = append(ids, id)
			}
		}
		out, err := ts.ApplyAPIKeyPolicyTemplate(req.Context(), templateID, ids)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "batch apply policy failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/api-keys/policy-audit", func(w http.ResponseWriter, req *http.Request) {
		ts, ok := svc.(interface {
			ListAPIKeyPolicyAudit(ctx context.Context, limit int, offset int) ([]map[string]any, error)
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "policy audit not supported")
			return
		}
		limit := parseIntQuery(req, "limit", 100)
		offset := parseIntQuery(req, "offset", 0)
		out, err := ts.ListAPIKeyPolicyAudit(req.Context(), limit, offset)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "list policy audit failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/auth/login", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		var payload schemas.LoginRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if payload.APIKey == "" {
			writeJSONError(w, http.StatusUnauthorized, "api_key is required")
			return
		}
		item, err := svc.ValidateAPIKey(req.Context(), payload.APIKey)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "invalid api key")
			return
		}
		if err := checkAPIKeyLifecycle(item); err != nil {
			writeJSONError(w, http.StatusForbidden, "API Key 已过期。")
			return
		}
		if !isIPAllowed(item.IPAllowlist, req) {
			writeJSONError(w, http.StatusForbidden, "请求来源 IP 不在允许范围内。")
			return
		}
		if item.QuotaTokensMonth != nil {
			if checker, ok := svc.(interface {
				CheckAPIKeyQuota(ctx context.Context, apiKeyID int64, quotaTokensMonthly int64) error
			}); ok {
				if err := checker.CheckAPIKeyQuota(req.Context(), item.ID, *item.QuotaTokensMonth); err != nil {
					writeJSONError(w, http.StatusForbidden, "API Key 月度配额已用尽。")
					return
				}
			}
		}
		sessionData, err := sessions.Create(item.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "create session failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"session_token": sessionData.Token,
			"api_key_id":    item.ID,
			"message":       "登录成功，请使用此 token 进行后续请求。使用 /auth/bind-model 绑定模型。",
		})
	})
	r.Post("/auth/bind-model", func(w http.ResponseWriter, req *http.Request) {
		token := extractBearerToken(req)
		if token == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		if _, ok := sessions.Get(token); !ok {
			writeJSONError(w, http.StatusNotFound, "session not found or expired")
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		providerName, _ := payload["provider_name"].(string)
		modelName, _ := payload["model_name"].(string)
		if strings.TrimSpace(providerName) == "" || strings.TrimSpace(modelName) == "" {
			writeJSONError(w, http.StatusBadRequest, "provider_name and model_name are required")
			return
		}
		bindingType, _ := payload["binding_type"].(string)
		if bindingType == "" {
			bindingType = "default"
		}
		if _, err := svc.GetModelByProviderAndName(req.Context(), providerName, modelName); err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "model not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "get model failed")
			return
		}
		if !sessions.BindProfileModel(token, providerName, modelName, bindingType) {
			writeJSONError(w, http.StatusNotFound, "session not found or expired")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"message":       fmt.Sprintf("模型 %s/%s 已绑定到 session (%s)", providerName, modelName, bindingType),
			"provider_name": providerName,
			"model_name":    modelName,
			"binding_type":  bindingType,
		})
	})
	r.Get("/auth/oauth/{provider}/authorize", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerType := strings.ToLower(strings.TrimSpace(chi.URLParam(req, "provider")))
		providerName := strings.TrimSpace(req.URL.Query().Get("provider_name"))
		if providerName == "" {
			providerName = providerType
		}
		callbackURL := strings.TrimSpace(req.URL.Query().Get("callback_url"))
		if callbackURL == "" || callbackURL == "/" {
			scheme := "http"
			if req.TLS != nil {
				scheme = "https"
			}
			callbackURL = scheme + "://" + req.Host
		}
		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		}
		backendBaseURL := scheme + "://" + req.Host
		accountName := strings.TrimSpace(req.URL.Query().Get("account_name"))
		setDefault := strings.EqualFold(strings.TrimSpace(req.URL.Query().Get("set_default")), "true")
		url, state, err := svc.OAuthAuthorizeURL(req.Context(), providerType, providerName, callbackURL, backendBaseURL, accountName, setDefault)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"url": url, "state": state})
	})
	r.Get("/auth/oauth/{provider}/callback", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerType := strings.ToLower(strings.TrimSpace(chi.URLParam(req, "provider")))
		code := strings.TrimSpace(req.URL.Query().Get("code"))
		state := strings.TrimSpace(req.URL.Query().Get("state"))
		if code == "" || state == "" {
			writeJSONError(w, http.StatusBadRequest, "missing code or state")
			return
		}
		redirectURL, err := svc.OAuthHandleCallback(req.Context(), providerType, code, state)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if strings.TrimSpace(redirectURL) == "" {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
		http.Redirect(w, req, redirectURL, http.StatusFound)
	})
	r.Get("/auth/oauth/{provider}/status", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerName := strings.TrimSpace(req.URL.Query().Get("provider_name"))
		if providerName == "" {
			providerName = strings.ToLower(strings.TrimSpace(chi.URLParam(req, "provider")))
		}
		has, err := svc.OAuthHasCredential(req.Context(), providerName)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"provider_name": providerName,
			"has_oauth":     has,
		})
	})
	r.Get("/auth/oauth/{provider}/accounts", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerName := strings.TrimSpace(req.URL.Query().Get("provider_name"))
		if providerName == "" {
			providerName = strings.ToLower(strings.TrimSpace(chi.URLParam(req, "provider")))
		}
		items, err := svc.ListOAuthAccounts(req.Context(), providerName)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"provider_name": providerName,
			"accounts":      items,
		})
	})
	r.Patch("/auth/oauth/{provider}/accounts/{account_id}", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerName := strings.TrimSpace(req.URL.Query().Get("provider_name"))
		if providerName == "" {
			providerName = strings.ToLower(strings.TrimSpace(chi.URLParam(req, "provider")))
		}
		accountID, err := parseIDParam(req, "account_id")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid account_id")
			return
		}
		var payload schemas.OAuthAccountUpdate
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		item, err := svc.UpdateOAuthAccount(req.Context(), providerName, accountID, payload)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "oauth account not found")
				return
			}
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	r.Post("/auth/oauth/{provider}/accounts/{account_id}/default", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerName := strings.TrimSpace(req.URL.Query().Get("provider_name"))
		if providerName == "" {
			providerName = strings.ToLower(strings.TrimSpace(chi.URLParam(req, "provider")))
		}
		accountID, err := parseIDParam(req, "account_id")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid account_id")
			return
		}
		item, err := svc.SetDefaultOAuthAccount(req.Context(), providerName, accountID)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "oauth account not found")
				return
			}
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	r.Delete("/auth/oauth/{provider}/accounts/{account_id}", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerName := strings.TrimSpace(req.URL.Query().Get("provider_name"))
		if providerName == "" {
			providerName = strings.ToLower(strings.TrimSpace(chi.URLParam(req, "provider")))
		}
		accountID, err := parseIDParam(req, "account_id")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid account_id")
			return
		}
		revoked, err := svc.RevokeOAuthAccount(req.Context(), providerName, accountID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"provider_name": providerName,
			"account_id":    accountID,
			"revoked":       revoked,
		})
	})
	r.Post("/auth/oauth/{provider}/revoke", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		providerName := strings.ToLower(strings.TrimSpace(chi.URLParam(req, "provider")))
		payload, err := readJSONBody(req, true)
		if err == nil {
			if v, ok := payload["provider_name"].(string); ok && strings.TrimSpace(v) != "" {
				providerName = strings.TrimSpace(v)
			}
		}
		revoked, err := svc.OAuthRevokeCredential(req.Context(), providerName)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"provider_name": providerName,
			"revoked":       revoked,
		})
	})
	r.Post("/auth/logout", func(w http.ResponseWriter, req *http.Request) {
		token := extractBearerToken(req)
		if token == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		if _, ok := sessions.Get(token); !ok {
			writeJSONError(w, http.StatusUnauthorized, "invalid session token")
			return
		}
		sessions.Delete(token)
		writeJSON(w, http.StatusOK, map[string]any{"message": "logout success"})
	})
	r.Post("/v1/chat/completions", func(w http.ResponseWriter, req *http.Request) {
		handleOpenAIChatCompletions(w, req, svc, "")
	})
	r.Post("/v1/responses", func(w http.ResponseWriter, req *http.Request) {
		handleOpenAIResponses(w, req, svc)
	})
	r.Post("/v1/embeddings", func(w http.ResponseWriter, req *http.Request) {
		handleOpenAIEmbeddings(w, req, svc)
	})
	r.Post("/v1/audio/speech", func(w http.ResponseWriter, req *http.Request) {
		handleOpenAIAudioSpeech(w, req, svc)
	})
	r.Post("/v1/audio/transcriptions", func(w http.ResponseWriter, req *http.Request) {
		handleOpenAIAudioRequest(w, req, svc, true)
	})
	r.Post("/v1/audio/translations", func(w http.ResponseWriter, req *http.Request) {
		handleOpenAIAudioRequest(w, req, svc, false)
	})
	r.Post("/v1/images/generations", func(w http.ResponseWriter, req *http.Request) {
		handleOpenAIImagesGenerations(w, req, svc)
	})
	r.Post("/v1/videos/generations", func(w http.ResponseWriter, req *http.Request) {
		handleOpenAIVideosGenerations(w, req, svc)
	})
	r.Get("/v1/videos/generations/{job_id}", func(w http.ResponseWriter, req *http.Request) {
		jobID := strings.TrimSpace(chi.URLParam(req, "job_id"))
		if jobID == "" {
			writeJSONError(w, http.StatusBadRequest, "job_id is required")
			return
		}
		item := videoJobStoreGet(jobID)
		if item == nil {
			writeJSONError(w, http.StatusNotFound, "video generation job not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	r.Get("/v1/realtime", func(w http.ResponseWriter, req *http.Request) {
		writeJSONError(w, http.StatusNotImplemented, "realtime websocket requires websocket client support")
	})
	r.Post("/{provider_name}/v1/chat/completions", func(w http.ResponseWriter, req *http.Request) {
		handleOpenAIChatCompletions(w, req, svc, chi.URLParam(req, "provider_name"))
	})
	r.Post("/route", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		model, provider, err := resolveRouteTarget(req.Context(), svc, payload)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "route target not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "route decision failed")
			return
		}
		baseURL := ""
		if provider.BaseURL != nil {
			baseURL = *provider.BaseURL
		}
		resp := map[string]any{
			"provider": provider.Name,
			"model":    provider.Name + "/" + model.Name,
			"base_url": baseURL,
		}
		if provider.APIKey != nil && *provider.APIKey != "" {
			resp["api_key"] = *provider.APIKey
		}
		writeJSON(w, http.StatusOK, resp)
	})
	r.Get("/route/pairs", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"default_pair": "",
			"pairs":        map[string]any{},
		})
	})
	r.Post("/route/invoke", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		modelValue, _ := payload["model"].(string)
		if modelValue == "" {
			model, provider, err := resolveRouteTarget(req.Context(), svc, payload)
			if err != nil {
				if errors.Is(err, services.ErrNotFound) {
					writeJSONError(w, http.StatusNotFound, "route target not found")
					return
				}
				writeJSONError(w, http.StatusInternalServerError, "route decision failed")
				return
			}
			payload["model"] = provider.Name + "/" + model.Name
		}
		out, err := svc.OpenAIChatCompletions(req.Context(), "", payload)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "model not found")
				return
			}
			if errors.Is(err, services.ErrNotImplemented) {
				writeJSONError(w, http.StatusNotImplemented, err.Error())
				return
			}
			writeJSONError(w, http.StatusBadGateway, "route invoke failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/v1beta/models/{model}:generateContent", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		modelName := chi.URLParam(req, "model")
		if modelName == "" {
			writeJSONError(w, http.StatusBadRequest, "model is required")
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		out, err := svc.GeminiGenerateContent(req.Context(), modelName, payload)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "model not found")
				return
			}
			if errors.Is(err, services.ErrNotImplemented) {
				writeJSONError(w, http.StatusNotImplemented, err.Error())
				return
			}
			writeJSONError(w, http.StatusBadGateway, "gemini generate content failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/v1beta/models/{model}:streamGenerateContent", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		modelName := chi.URLParam(req, "model")
		if modelName == "" {
			writeJSONError(w, http.StatusBadRequest, "model is required")
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		streamResp, err := openStreamWithRetry(req.Context(), func() (*services.StreamResponse, error) {
			return svc.GeminiStreamGenerateContent(req.Context(), modelName, payload)
		})
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "model not found")
				return
			}
			if errors.Is(err, services.ErrNotImplemented) {
				writeJSONError(w, http.StatusNotImplemented, err.Error())
				return
			}
			writeJSONError(w, http.StatusBadGateway, "gemini stream generate content failed")
			return
		}
		defer streamResp.Body.Close()
		streamGeminiSSE(req.Context(), w, streamResp)
	})
	r.Post("/v1/messages", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		out, err := svc.ClaudeMessages(req.Context(), payload)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "model not found")
				return
			}
			if errors.Is(err, services.ErrNotImplemented) {
				writeJSONError(w, http.StatusNotImplemented, err.Error())
				return
			}
			writeJSONError(w, http.StatusBadGateway, "claude messages failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/v1/messages/count_tokens", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		out, err := svc.ClaudeCountTokens(req.Context(), payload)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, "claude count_tokens failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/v1/messages/batches", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		out, err := svc.ClaudeCreateMessageBatch(req.Context(), payload)
		if err != nil {
			var upstreamErr *services.UpstreamStatusError
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "claude provider not found")
				return
			}
			if errors.As(err, &upstreamErr) {
				writeJSONError(w, upstreamErr.StatusCode, upstreamErr.Detail)
				return
			}
			writeJSONError(w, http.StatusBadRequest, "claude message batch create failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/v1/messages/batches/{batch_id}", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		batchID := strings.TrimSpace(chi.URLParam(req, "batch_id"))
		if batchID == "" {
			writeJSONError(w, http.StatusBadRequest, "batch_id is required")
			return
		}
		out, err := svc.ClaudeGetMessageBatch(req.Context(), batchID)
		if err != nil {
			var upstreamErr *services.UpstreamStatusError
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "claude batch not found")
				return
			}
			if errors.As(err, &upstreamErr) {
				writeJSONError(w, upstreamErr.StatusCode, upstreamErr.Detail)
				return
			}
			writeJSONError(w, http.StatusBadRequest, "claude message batch get failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/v1/messages/batches/{batch_id}/cancel", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		batchID := strings.TrimSpace(chi.URLParam(req, "batch_id"))
		if batchID == "" {
			writeJSONError(w, http.StatusBadRequest, "batch_id is required")
			return
		}
		out, err := svc.ClaudeCancelMessageBatch(req.Context(), batchID)
		if err != nil {
			var upstreamErr *services.UpstreamStatusError
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "claude batch not found")
				return
			}
			if errors.As(err, &upstreamErr) {
				writeJSONError(w, upstreamErr.StatusCode, upstreamErr.Detail)
				return
			}
			writeJSONError(w, http.StatusBadRequest, "claude message batch cancel failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/monitor/invocations", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		limit := parseIntQuery(req, "limit", 50)
		offset := parseIntQuery(req, "offset", 0)
		if querySvc, ok := svc.(interface {
			QueryInvocations(ctx context.Context, opts services.InvocationQueryOptions) ([]schemas.MonitorInvocation, error)
		}); ok {
			opts := services.InvocationQueryOptions{
				Limit:        limit,
				Offset:       offset,
				ModelName:    strings.TrimSpace(req.URL.Query().Get("model_name")),
				ProviderName: strings.TrimSpace(req.URL.Query().Get("provider_name")),
				Status:       strings.TrimSpace(req.URL.Query().Get("status")),
				AuthType:     strings.TrimSpace(req.URL.Query().Get("auth_type")),
				OrderBy:      strings.TrimSpace(req.URL.Query().Get("order_by")),
				OrderDesc:    strings.ToLower(strings.TrimSpace(req.URL.Query().Get("order_desc"))) != "false",
			}
			if raw := strings.TrimSpace(req.URL.Query().Get("model_id")); raw != "" {
				var id int64
				if _, err := fmt.Sscanf(raw, "%d", &id); err == nil && id > 0 {
					opts.ModelID = &id
				}
			}
			if raw := strings.TrimSpace(req.URL.Query().Get("provider_id")); raw != "" {
				var id int64
				if _, err := fmt.Sscanf(raw, "%d", &id); err == nil && id > 0 {
					opts.ProviderID = &id
				}
			}
			if raw := strings.TrimSpace(req.URL.Query().Get("api_key_id")); raw != "" {
				var id int64
				if _, err := fmt.Sscanf(raw, "%d", &id); err == nil && id > 0 {
					opts.APIKeyID = &id
				}
			}
			if raw := strings.TrimSpace(req.URL.Query().Get("start_time")); raw != "" {
				if ts, err := time.Parse(time.RFC3339, raw); err == nil {
					opts.StartTime = &ts
				}
			}
			if raw := strings.TrimSpace(req.URL.Query().Get("end_time")); raw != "" {
				if ts, err := time.Parse(time.RFC3339, raw); err == nil {
					opts.EndTime = &ts
				}
			}
			out, err := querySvc.QueryInvocations(req.Context(), opts)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "list monitor invocations failed")
				return
			}
			writeJSON(w, http.StatusOK, out)
			return
		}
		out, err := svc.ListInvocations(req.Context(), limit, offset)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "list monitor invocations failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/monitor/invocations/{id}", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		id, err := parseIDParam(req, "id")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid id")
			return
		}
		out, err := svc.GetInvocation(req.Context(), id)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "invocation not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "get monitor invocation failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/monitor/login-records", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		limit := parseIntQuery(req, "limit", 100)
		if limit > 500 {
			limit = 500
		}
		offset := parseIntQuery(req, "offset", 0)
		out, total, err := svc.ListLoginRecords(req.Context(), limit, offset)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"records":          []any{},
				"total":            0,
				"redis_available":  false,
				"storage_fallback": "none",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"records":         out,
			"total":           total,
			"redis_available": false,
		})
	})
	r.Get("/monitor/statistics", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		out, err := svc.GetInvocationStatistics(req.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "get monitor statistics failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/monitor/quota-details", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		qs, ok := svc.(interface {
			GetQuotaDetails(ctx context.Context, q services.QuotaDetailQuery) ([]map[string]any, error)
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "quota details not supported")
			return
		}
		q := services.QuotaDetailQuery{
			ProviderName: strings.TrimSpace(req.URL.Query().Get("provider_name")),
			ModelName:    strings.TrimSpace(req.URL.Query().Get("model_name")),
			Limit:        parseIntQuery(req, "limit", 100),
			Offset:       parseIntQuery(req, "offset", 0),
		}
		if raw := strings.TrimSpace(req.URL.Query().Get("api_key_id")); raw != "" {
			var id int64
			if _, err := fmt.Sscanf(raw, "%d", &id); err == nil && id > 0 {
				q.APIKeyID = &id
			}
		}
		if raw := strings.TrimSpace(req.URL.Query().Get("start_time")); raw != "" {
			if ts, err := time.Parse(time.RFC3339, raw); err == nil {
				q.StartTime = &ts
			}
		}
		if raw := strings.TrimSpace(req.URL.Query().Get("end_time")); raw != "" {
			if ts, err := time.Parse(time.RFC3339, raw); err == nil {
				q.EndTime = &ts
			}
		}
		out, err := qs.GetQuotaDetails(req.Context(), q)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "get quota details failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/monitor/quota-details/export", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		qs, ok := svc.(interface {
			GetQuotaDetails(ctx context.Context, q services.QuotaDetailQuery) ([]map[string]any, error)
			ExportQuotaDetailsCSV(ctx context.Context, q services.QuotaDetailQuery) ([]byte, error)
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "quota details export not supported")
			return
		}
		q := services.QuotaDetailQuery{
			ProviderName: strings.TrimSpace(req.URL.Query().Get("provider_name")),
			ModelName:    strings.TrimSpace(req.URL.Query().Get("model_name")),
			Limit:        parseIntQuery(req, "limit", 1000),
			Offset:       parseIntQuery(req, "offset", 0),
		}
		format := strings.ToLower(strings.TrimSpace(req.URL.Query().Get("format")))
		if format == "json" {
			out, err := qs.GetQuotaDetails(req.Context(), q)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "export quota details json failed")
				return
			}
			writeJSON(w, http.StatusOK, out)
			return
		}
		data, err := qs.ExportQuotaDetailsCSV(req.Context(), q)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "export quota details csv failed")
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="quota_details.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	r.Get("/monitor/budget-alerts", func(w http.ResponseWriter, req *http.Request) {
		qs, ok := svc.(interface {
			GetBudgetAlerts(ctx context.Context) (map[string]any, error)
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "budget alerts not supported")
			return
		}
		out, err := qs.GetBudgetAlerts(req.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "get budget alerts failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Put("/monitor/budget-alerts", func(w http.ResponseWriter, req *http.Request) {
		qs, ok := svc.(interface {
			UpdateBudgetAlerts(ctx context.Context, day, week, month int64) (map[string]any, error)
		})
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "budget alerts not supported")
			return
		}
		payload, err := readJSONBody(req, false)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		day, _ := toInt64(payload["day_tokens"])
		week, _ := toInt64(payload["week_tokens"])
		month, _ := toInt64(payload["month_tokens"])
		out, err := qs.UpdateBudgetAlerts(req.Context(), day, week, month)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "update budget alerts failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/monitor/channel-load", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		type channelLoadReader interface {
			GetChannelLoadSnapshot() map[string]any
		}
		reader, ok := svc.(channelLoadReader)
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "channel load snapshot is not supported by current catalog service")
			return
		}
		writeJSON(w, http.StatusOK, reader.GetChannelLoadSnapshot())
	})
	r.Get("/monitor/time-series", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		granularity := strings.TrimSpace(req.URL.Query().Get("granularity"))
		if granularity == "" {
			granularity = "day"
		}
		timeRangeHours := parseIntQuery(req, "time_range_hours", 168)
		if timeRangeHours <= 0 {
			writeJSONError(w, http.StatusBadRequest, "time_range_hours must be positive")
			return
		}
		out, err := svc.GetMonitorTimeSeries(req.Context(), granularity, timeRangeHours)
		if err != nil {
			if strings.Contains(err.Error(), "unsupported granularity") {
				writeJSONError(w, http.StatusBadRequest, err.Error())
				return
			}
			if strings.Contains(err.Error(), "time_range_hours must be positive") {
				writeJSONError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "get monitor time series failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/monitor/grouped-time-series", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		groupBy := strings.TrimSpace(req.URL.Query().Get("group_by"))
		if groupBy == "" {
			groupBy = "model"
		}
		granularity := strings.TrimSpace(req.URL.Query().Get("granularity"))
		if granularity == "" {
			granularity = "day"
		}
		timeRangeHours := parseIntQuery(req, "time_range_hours", 168)
		if timeRangeHours <= 0 {
			writeJSONError(w, http.StatusBadRequest, "time_range_hours must be positive")
			return
		}
		out, err := svc.GetMonitorGroupedTimeSeries(req.Context(), groupBy, granularity, timeRangeHours)
		if err != nil {
			if strings.Contains(err.Error(), "unsupported group_by") || strings.Contains(err.Error(), "unsupported granularity") {
				writeJSONError(w, http.StatusBadRequest, err.Error())
				return
			}
			if strings.Contains(err.Error(), "time_range_hours must be positive") {
				writeJSONError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "get monitor grouped time series failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/monitor/export/json", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		limit := parseIntQuery(req, "limit", 1000)
		offset := parseIntQuery(req, "offset", 0)
		out, err := svc.ListInvocations(req.Context(), limit, offset)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "export monitor json failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/monitor/export/excel", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		limit := parseIntQuery(req, "limit", 1000)
		offset := parseIntQuery(req, "offset", 0)
		data, err := svc.ExportInvocationsCSV(req.Context(), limit, offset)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "export monitor excel failed")
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="monitor_invocations.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	r.Get("/monitor/database", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		format := strings.ToLower(strings.TrimSpace(req.URL.Query().Get("format")))
		if format == "zip" {
			limit := parseIntQuery(req, "limit", 1000)
			offset := parseIntQuery(req, "offset", 0)
			items, err := svc.ListInvocations(req.Context(), limit, offset)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "query monitor invocations failed")
				return
			}
			csvData, err := svc.ExportInvocationsCSV(req.Context(), limit, offset)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "export monitor csv failed")
				return
			}
			zipData, err := buildMonitorExportZip(items, csvData)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "build monitor export zip failed")
				return
			}
			filename := fmt.Sprintf("llm_router_monitor_export_%s.zip", time.Now().UTC().Format("20060102_150405"))
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(zipData)
			return
		}

		data, err := svc.ExportMonitorDatabaseSQLite(req.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "export monitor database failed")
			return
		}
		w.Header().Set("Content-Type", "application/x-sqlite3")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Header().Set("Content-Disposition", `attachment; filename="llm_datas.db"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	r.Get("/pricing/latest", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		out, err := svc.GetLatestPricing(req.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "get latest pricing failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Get("/pricing/suggestions", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		out, err := svc.GetPricingSuggestions(req.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "get pricing suggestions failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/pricing/sync/{model_id}", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		modelID, err := parseIDParam(req, "model_id")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid model_id")
			return
		}
		out, err := svc.SyncModelPricing(req.Context(), modelID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/pricing/sync-all", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		out, err := svc.SyncAllPricing(req.Context())
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/config/sync", func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		hint := opts.ModelConfigHintPath
		if strings.TrimSpace(hint) == "" {
			hint = "router.toml"
		}
		resolved, err := config.ResolveModelConfigPath(hint)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, fmt.Sprintf("model config not found: %v", err))
			return
		}
		if err := svc.SyncRouterTOML(req.Context(), resolved); err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("sync failed: %v", err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success":     true,
			"message":     "配置已从 router.toml 同步到数据库",
			"config_file": resolved,
		})
	})
}

func authMiddleware(svc CatalogService, sessions SessionStore, opts RouterOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if isPublicPath(req.URL.Path) {
				next.ServeHTTP(w, req)
				return
			}
			if opts.AllowLocalNoAuth && isLocalRequest(req) {
				next.ServeHTTP(w, req)
				return
			}

			token := extractBearerToken(req)
			if token == "" {
				token = strings.TrimSpace(req.Header.Get("X-API-Key"))
			}
			if token == "" {
				writeJSONError(w, http.StatusUnauthorized, "未认证。请先通过 /auth/login 登录获取 Session Token，或使用 API Key 进行认证。")
				return
			}

			if _, ok := sessions.Get(token); ok {
				sessionData, _ := sessions.Get(token)
				if svc == nil {
					writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
					return
				}
				item, err := svc.GetAPIKey(req.Context(), sessionData.APIKeyID)
				if err != nil {
					writeJSONError(w, http.StatusForbidden, "API Key 无效或已禁用。")
					return
				}
				if err := checkAPIKeyLifecycle(item); err != nil {
					writeJSONError(w, http.StatusForbidden, "API Key 已过期。")
					return
				}
				if !isIPAllowed(item.IPAllowlist, req) {
					writeJSONError(w, http.StatusForbidden, "请求来源 IP 不在允许范围内。")
					return
				}
				if item.QuotaTokensMonth != nil {
					if checker, ok := svc.(interface {
						CheckAPIKeyQuota(ctx context.Context, apiKeyID int64, quotaTokensMonthly int64) error
					}); ok {
						if err := checker.CheckAPIKeyQuota(req.Context(), item.ID, *item.QuotaTokensMonth); err != nil {
							writeJSONError(w, http.StatusForbidden, "API Key 月度配额已用尽。")
							return
						}
					}
				}
				ctx := withAuthContext(req.Context(), authContextData{
					APIKey:   item,
					Policy:   services.NewAPIKeyPolicy(item),
					AuthType: "session_token",
				})
				next.ServeHTTP(w, req.WithContext(ctx))
				return
			}
			if svc == nil {
				writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
				return
			}
			if item, err := svc.ValidateAPIKey(req.Context(), token); err == nil {
				if err := checkAPIKeyLifecycle(item); err != nil {
					writeJSONError(w, http.StatusForbidden, "API Key 已过期。")
					return
				}
				if !isIPAllowed(item.IPAllowlist, req) {
					writeJSONError(w, http.StatusForbidden, "请求来源 IP 不在允许范围内。")
					return
				}
				if item.QuotaTokensMonth != nil {
					if checker, ok := svc.(interface {
						CheckAPIKeyQuota(ctx context.Context, apiKeyID int64, quotaTokensMonthly int64) error
					}); ok {
						if err := checker.CheckAPIKeyQuota(req.Context(), item.ID, *item.QuotaTokensMonth); err != nil {
							writeJSONError(w, http.StatusForbidden, "API Key 月度配额已用尽。")
							return
						}
					}
				}
				ctx := withAuthContext(req.Context(), authContextData{
					APIKey:   item,
					Policy:   services.NewAPIKeyPolicy(item),
					AuthType: "api_key",
				})
				next.ServeHTTP(w, req.WithContext(ctx))
				return
			}
			writeJSONError(w, http.StatusForbidden, "API Key 无效或已禁用。")
		})
	}
}

func isPublicPath(path string) bool {
	return path == "/health" ||
		path == "/api/health" ||
		path == "/auth/login" ||
		path == "/api/auth/login" ||
		strings.HasPrefix(path, "/auth/oauth/") ||
		strings.HasPrefix(path, "/api/auth/oauth/")
}

func isLocalRequest(req *http.Request) bool {
	host := strings.TrimSpace(req.Header.Get("X-Forwarded-For"))
	if host == "" {
		host = req.RemoteAddr
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]any{"detail": detail})
}

func writeInvokeError(w http.ResponseWriter, err error, fallbackDetail string) {
	var upstreamErr *services.UpstreamStatusError
	if errors.As(err, &upstreamErr) {
		status := upstreamErr.StatusCode
		if status < 400 || status > 599 {
			status = http.StatusBadGateway
		}
		detail := strings.TrimSpace(upstreamErr.Detail)
		if detail == "" {
			detail = fallbackDetail
		}
		writeJSONError(w, status, detail)
		return
	}
	writeJSONError(w, http.StatusBadGateway, fallbackDetail)
}

func parseIDParam(req *http.Request, key string) (int64, error) {
	raw := chi.URLParam(req, key)
	if raw == "" {
		return 0, errors.New("missing param")
	}
	var id int64
	_, err := fmt.Sscanf(raw, "%d", &id)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func handleOpenAIChatCompletions(w http.ResponseWriter, req *http.Request, svc CatalogService, providerHint string) {
	if svc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
		return
	}
	var payload map[string]any
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	model, ok := payload["model"].(string)
	if !ok || model == "" {
		writeJSONError(w, http.StatusBadRequest, "model is required")
		return
	}
	payload, status, detail := enforcePayloadPolicy(req, providerHint, payload)
	if status != 0 {
		writeJSONError(w, status, detail)
		return
	}
	if stream, ok := payload["stream"].(bool); ok && stream {
		ensureStreamOptionsIncludeUsage(payload)
		streamResp, err := openStreamWithRetry(req.Context(), func() (*services.StreamResponse, error) {
			return svc.OpenAIChatCompletionsStream(req.Context(), providerHint, payload)
		})
		if err != nil {
			var upstreamErr *services.UpstreamStatusError
			switch {
			case errors.Is(err, services.ErrNotFound):
				writeJSONError(w, http.StatusNotFound, "model not found")
			case errors.Is(err, services.ErrNotImplemented):
				writeJSONError(w, http.StatusNotImplemented, err.Error())
			case errors.As(err, &upstreamErr):
				writeJSONError(w, upstreamErr.StatusCode, upstreamErr.Detail)
			default:
				writeJSONError(w, http.StatusBadGateway, "stream chat completion invoke failed")
			}
			return
		}
		defer streamResp.Body.Close()
		streamSSEAndTrackUsage(req, svc, w, streamResp, 1)
		return
	}
	out, err := svc.OpenAIChatCompletions(req.Context(), providerHint, payload)
	if err != nil {
		var upstreamErr *services.UpstreamStatusError
		switch {
		case errors.Is(err, services.ErrNotFound):
			writeJSONError(w, http.StatusNotFound, "model not found")
		case errors.Is(err, services.ErrNotImplemented):
			writeJSONError(w, http.StatusNotImplemented, err.Error())
		case errors.As(err, &upstreamErr):
			writeJSONError(w, upstreamErr.StatusCode, upstreamErr.Detail)
		default:
			writeJSONError(w, http.StatusBadGateway, "chat completion invoke failed")
		}
		return
	}
	maybeAccumulateUsage(req, svc, out, 0)
	writeJSON(w, http.StatusOK, out)
}

func streamSSE(ctx context.Context, w http.ResponseWriter, streamResp *services.StreamResponse) {
	contentType := streamResp.ContentType
	if strings.TrimSpace(contentType) == "" {
		contentType = "text/event-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	reader := bufio.NewReader(streamResp.Body)
	for {
		line, err := readLineWithTimeout(ctx, reader, streamIdleTimeout)
		if line != "" {
			if _, writeErr := io.WriteString(w, line); writeErr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				_, _ = io.WriteString(w, "data: {\"error\":\"stream_timeout\"}\n\n")
				if flusher != nil {
					flusher.Flush()
				}
				return
			}
			return
		}
	}
}

type streamUsageTracker struct {
	tokens int64
	cost   float64
}

func (t *streamUsageTracker) ObserveSSELine(line string) {
	if t == nil {
		return
	}
	payload, ok := parseSSEDataPayload(line)
	if !ok {
		return
	}
	tokens := extractUsageTokens(payload)
	if tokens > 0 {
		t.tokens = tokens
	}
	cost := extractUsageCost(payload)
	if cost > 0 {
		t.cost = cost
	}
}

func (t *streamUsageTracker) Snapshot() map[string]any {
	if t == nil {
		return nil
	}
	out := map[string]any{}
	if t.tokens > 0 {
		out["usage"] = map[string]any{"total_tokens": t.tokens}
	}
	if t.cost > 0 {
		out["cost"] = t.cost
	}
	return out
}

func parseSSEDataPayload(line string) (map[string]any, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "data:") {
		return nil, false
	}
	payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if payload == "" || payload == "[DONE]" {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(payload), &out); err != nil {
		return nil, false
	}
	return out, true
}

func ensureStreamOptionsIncludeUsage(payload map[string]any) {
	if payload == nil {
		return
	}
	raw, ok := payload["stream_options"]
	if !ok || raw == nil {
		payload["stream_options"] = map[string]any{"include_usage": true}
		return
	}
	if options, ok := raw.(map[string]any); ok {
		options["include_usage"] = true
		payload["stream_options"] = options
		return
	}
}

func streamSSEAndTrackUsage(req *http.Request, svc CatalogService, w http.ResponseWriter, streamResp *services.StreamResponse, fallbackTokens int64) {
	tracker := &streamUsageTracker{}
	defer func() {
		maybeAccumulateUsage(req, svc, tracker.Snapshot(), fallbackTokens)
	}()
	contentType := streamResp.ContentType
	if strings.TrimSpace(contentType) == "" {
		contentType = "text/event-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	reader := bufio.NewReader(streamResp.Body)
	for {
		line, err := readLineWithTimeout(req.Context(), reader, streamIdleTimeout)
		if line != "" {
			tracker.ObserveSSELine(line)
			if _, writeErr := io.WriteString(w, line); writeErr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				_, _ = io.WriteString(w, "data: {\"error\":\"stream_timeout\"}\n\n")
				if flusher != nil {
					flusher.Flush()
				}
				return
			}
			return
		}
	}
}

func streamGeminiSSE(ctx context.Context, w http.ResponseWriter, streamResp *services.StreamResponse) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	reader := bufio.NewReader(streamResp.Body)
	for {
		line, err := readLineWithTimeout(ctx, reader, streamIdleTimeout)
		if line != "" {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "data:") {
				payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				converted, handled := convertOpenAIStreamChunkToGemini(payload)
				if handled {
					_, _ = w.Write([]byte("data: " + converted + "\n\n"))
				} else {
					_, _ = w.Write([]byte(line))
				}
			} else {
				_, _ = w.Write([]byte(line))
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				break
			}
			if errors.Is(err, context.DeadlineExceeded) {
				_, _ = io.WriteString(w, "data: {\"error\":\"stream_timeout\"}\n\n")
				if flusher != nil {
					flusher.Flush()
				}
			}
			return
		}
	}
}

type readLineResult struct {
	line string
	err  error
}

func readLineWithTimeout(ctx context.Context, reader *bufio.Reader, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	ch := make(chan readLineResult, 1)
	go func() {
		line, err := reader.ReadString('\n')
		ch <- readLineResult{line: line, err: err}
	}()
	select {
	case <-ctx.Done():
		return "", context.Canceled
	case <-time.After(timeout):
		return "", context.DeadlineExceeded
	case out := <-ch:
		return out.line, out.err
	}
}

func openStreamWithRetry(ctx context.Context, openFn func() (*services.StreamResponse, error)) (*services.StreamResponse, error) {
	var lastErr error
	for attempt := 0; attempt < streamOpenRetryAttempts; attempt++ {
		streamResp, err := openFn()
		if err == nil {
			return streamResp, nil
		}
		lastErr = err
		if !shouldRetryStreamOpenError(err) || attempt == streamOpenRetryAttempts-1 {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, context.Canceled
		case <-time.After(streamOpenRetryBackoff):
		}
	}
	return nil, lastErr
}

func shouldRetryStreamOpenError(err error) bool {
	var upstreamErr *services.UpstreamStatusError
	if errors.As(err, &upstreamErr) {
		return upstreamErr.StatusCode == http.StatusTooManyRequests || upstreamErr.StatusCode >= 500
	}
	return true
}

func convertOpenAIStreamChunkToGemini(payload string) (string, bool) {
	if payload == "" {
		return "", false
	}
	if payload == "[DONE]" {
		return "[DONE]", true
	}
	var chunk map[string]any
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return "", false
	}
	text := ""
	if choices, ok := chunk["choices"].([]any); ok && len(choices) > 0 {
		if choice0, ok := choices[0].(map[string]any); ok {
			if delta, ok := choice0["delta"].(map[string]any); ok {
				if content, ok := delta["content"].(string); ok {
					text = content
				}
			}
		}
	}
	if text == "" {
		return "", false
	}
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
			},
		},
	}
	if usage, ok := chunk["usage"].(map[string]any); ok {
		out["usageMetadata"] = map[string]any{
			"promptTokenCount":     usage["prompt_tokens"],
			"candidatesTokenCount": usage["completion_tokens"],
			"totalTokenCount":      usage["total_tokens"],
		}
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", false
	}
	return string(raw), true
}

func buildMonitorExportZip(items []schemas.MonitorInvocation, csvData []byte) ([]byte, error) {
	jsonData, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("marshal monitor invocations json: %w", err)
	}
	metadataRaw, err := json.Marshal(map[string]any{
		"format":       "llm-router-monitor-export-v1",
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"records":      len(items),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal monitor export metadata: %w", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := writeZipEntry(zw, "monitor_invocations.csv", csvData); err != nil {
		return nil, err
	}
	if err := writeZipEntry(zw, "monitor_invocations.json", jsonData); err != nil {
		return nil, err
	}
	if err := writeZipEntry(zw, "metadata.json", metadataRaw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close monitor export zip: %w", err)
	}
	return buf.Bytes(), nil
}

func writeZipEntry(zw *zip.Writer, name string, data []byte) error {
	writer, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", name, err)
	}
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("write zip entry %s: %w", name, err)
	}
	return nil
}

func resolveRouteTarget(ctx context.Context, svc CatalogService, payload map[string]any) (schemas.Model, schemas.Provider, error) {
	modelHint, _ := payload["model_hint"].(string)
	if modelHint == "" {
		modelHint, _ = payload["model"].(string)
	}
	if modelHint != "" {
		if providerName, modelName, ok := splitProviderModel(modelHint); ok {
			model, err := svc.GetModelByProviderAndName(ctx, providerName, modelName)
			if err == nil {
				provider, err := svc.GetProviderByName(ctx, providerName)
				if err != nil {
					return schemas.Model{}, schemas.Provider{}, err
				}
				return model, provider, nil
			}
		}
	}

	models, err := svc.ListModels(ctx)
	if err != nil {
		return schemas.Model{}, schemas.Provider{}, err
	}
	if len(models) == 0 {
		return schemas.Model{}, schemas.Provider{}, services.ErrNotFound
	}
	model := models[0]
	provider, err := svc.GetProviderByName(ctx, model.ProviderName)
	if err != nil {
		return schemas.Model{}, schemas.Provider{}, err
	}
	return model, provider, nil
}

func splitProviderModel(v string) (string, string, bool) {
	for i := 0; i < len(v); i++ {
		if v[i] == '/' {
			if i == 0 || i == len(v)-1 {
				return "", "", false
			}
			return v[:i], v[i+1:], true
		}
	}
	return "", "", false
}

func enforcePayloadPolicy(req *http.Request, providerHint string, payload map[string]any) (map[string]any, int, string) {
	authData, ok := getAuthContext(req.Context())
	if !ok {
		return payload, 0, ""
	}
	modelRaw, _ := payload["model"].(string)
	providerName, modelName := extractProviderModel(providerHint, modelRaw)
	if strings.TrimSpace(modelName) == "" {
		return nil, http.StatusBadRequest, "model is required"
	}
	if !authData.Policy.IsModelAllowed(providerName, modelName) {
		return nil, http.StatusForbidden, "API Key 策略不允许调用该模型或渠道。"
	}
	applyParameterLimits(payload, authData.Policy.ParameterLimits)
	return payload, 0, ""
}

func maybeAccumulateUsage(req *http.Request, svc CatalogService, out map[string]any, fallbackTokens int64) {
	authData, ok := getAuthContext(req.Context())
	if !ok || authData.APIKey.ID <= 0 || svc == nil {
		return
	}
	tracker, ok := svc.(interface {
		AccumulateAPIKeyUsage(ctx context.Context, apiKeyID int64, tokens int64, cost float64) error
	})
	if !ok {
		return
	}
	tokens := extractUsageTokens(out)
	if tokens <= 0 {
		tokens = fallbackTokens
	}
	cost := extractUsageCost(out)
	_ = tracker.AccumulateAPIKeyUsage(req.Context(), authData.APIKey.ID, tokens, cost)
}

func extractUsageTokens(out map[string]any) int64 {
	if out == nil {
		return 0
	}
	if usage, ok := out["usage"].(map[string]any); ok {
		if v, ok := toInt64(usage["total_tokens"]); ok {
			return v
		}
		p, _ := toInt64(usage["prompt_tokens"])
		c, _ := toInt64(usage["completion_tokens"])
		return p + c
	}
	if usageMeta, ok := out["usageMetadata"].(map[string]any); ok {
		if v, ok := toInt64(usageMeta["totalTokenCount"]); ok {
			return v
		}
	}
	return 0
}

func extractUsageCost(out map[string]any) float64 {
	if out == nil {
		return 0
	}
	if v, ok := out["cost"]; ok {
		if f, ok := toFloat64(v); ok {
			return f
		}
	}
	return 0
}

func toInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case float32:
		return int64(t), true
	case float64:
		return int64(t), true
	default:
		return 0, false
	}
}

func toFloat64(v any) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case float32:
		return float64(t), true
	case float64:
		return t, true
	default:
		return 0, false
	}
}

func parseIntQuery(req *http.Request, key string, defaultValue int) int {
	raw := strings.TrimSpace(req.URL.Query().Get(key))
	if raw == "" {
		return defaultValue
	}
	var v int
	if _, err := fmt.Sscanf(raw, "%d", &v); err != nil {
		return defaultValue
	}
	return v
}

var (
	videoJobsMu sync.RWMutex
	videoJobs   = map[string]map[string]any{}
)

func videoJobStoreSet(id string, item map[string]any) {
	videoJobsMu.Lock()
	videoJobs[id] = item
	videoJobsMu.Unlock()
}

func videoJobStoreGet(id string) map[string]any {
	videoJobsMu.RLock()
	item := videoJobs[id]
	videoJobsMu.RUnlock()
	return item
}

func handleOpenAIResponses(w http.ResponseWriter, req *http.Request, svc CatalogService) {
	if svc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
		return
	}
	payload, err := readJSONBody(req, false)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	model, _ := payload["model"].(string)
	input := payload["input"]
	instructions, _ := payload["instructions"].(string)
	if strings.TrimSpace(model) == "" {
		writeJSONError(w, http.StatusBadRequest, "model is required")
		return
	}
	messages := responsesInputToMessages(input, instructions)
	if len(messages) == 0 {
		writeJSONError(w, http.StatusBadRequest, "input is required")
		return
	}
	chatPayload := map[string]any{
		"model":    model,
		"messages": messages,
	}
	if v, ok := payload["temperature"]; ok {
		chatPayload["temperature"] = v
	}
	if v, ok := payload["top_p"]; ok {
		chatPayload["top_p"] = v
	}
	if v, ok := payload["max_output_tokens"]; ok {
		chatPayload["max_tokens"] = v
	}
	if stream, ok := payload["stream"].(bool); ok && stream {
		chatPayload["stream"] = true
		ensureStreamOptionsIncludeUsage(chatPayload)
		chatPayload, status, detail := enforcePayloadPolicy(req, "", chatPayload)
		if status != 0 {
			writeJSONError(w, status, detail)
			return
		}
		streamResp, err := openStreamWithRetry(req.Context(), func() (*services.StreamResponse, error) {
			return svc.OpenAIChatCompletionsStream(req.Context(), "", chatPayload)
		})
		if err != nil {
			writeInvokeError(w, err, "responses stream failed")
			return
		}
		defer streamResp.Body.Close()
		streamSSEAndTrackUsage(req, svc, w, streamResp, 1)
		return
	}
	chatPayload, status, detail := enforcePayloadPolicy(req, "", chatPayload)
	if status != 0 {
		writeJSONError(w, status, detail)
		return
	}
	out, err := svc.OpenAIResponses(req.Context(), "", chatPayload)
	if err != nil {
		writeInvokeError(w, err, "responses invoke failed")
		return
	}
	maybeAccumulateUsage(req, svc, out, 0)
	writeJSON(w, http.StatusOK, out)
}

func responsesInputToMessages(input any, instructions string) []map[string]any {
	messages := make([]map[string]any, 0)
	if strings.TrimSpace(instructions) != "" {
		messages = append(messages, map[string]any{"role": "system", "content": instructions})
	}
	appendUser := func(text string) {
		if strings.TrimSpace(text) != "" {
			messages = append(messages, map[string]any{"role": "user", "content": text})
		}
	}
	switch v := input.(type) {
	case string:
		appendUser(v)
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				role, _ := m["role"].(string)
				content := extractResponsesContent(m["content"])
				if strings.TrimSpace(content) == "" {
					continue
				}
				if role == "" {
					role = "user"
				}
				messages = append(messages, map[string]any{"role": role, "content": content})
				continue
			}
			appendUser(extractResponsesContent(item))
		}
	case map[string]any:
		role, _ := v["role"].(string)
		content := extractResponsesContent(v["content"])
		if role == "" {
			role = "user"
		}
		if strings.TrimSpace(content) != "" {
			messages = append(messages, map[string]any{"role": role, "content": content})
		}
	default:
		appendUser(fmt.Sprintf("%v", v))
	}
	return messages
}

func extractResponsesContent(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if m, ok := item.(map[string]any); ok {
				if text, ok := m["text"].(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprintf("%v", t)
	}
}

func handleOpenAIEmbeddings(w http.ResponseWriter, req *http.Request, svc CatalogService) {
	payload, err := readJSONBody(req, false)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload, status, detail := enforcePayloadPolicy(req, "", payload)
	if status != 0 {
		writeJSONError(w, status, detail)
		return
	}
	out, err := svc.OpenAIEmbeddings(req.Context(), "", payload)
	if err != nil {
		writeInvokeError(w, err, "embeddings invoke failed")
		return
	}
	maybeAccumulateUsage(req, svc, out, 0)
	writeJSON(w, http.StatusOK, out)
}

func handleOpenAIAudioSpeech(w http.ResponseWriter, req *http.Request, svc CatalogService) {
	payload, err := readJSONBody(req, false)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload, status, detail := enforcePayloadPolicy(req, "", payload)
	if status != 0 {
		writeJSONError(w, status, detail)
		return
	}
	data, contentType, err := svc.OpenAIAudioSpeech(req.Context(), "", payload)
	if err != nil {
		writeInvokeError(w, err, "audio speech invoke failed")
		return
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "audio/mpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func parseAudioRequest(req *http.Request) (map[string]any, []byte, string, string, error) {
	payload := map[string]any{}
	contentType := strings.ToLower(strings.TrimSpace(req.Header.Get("Content-Type")))
	if strings.Contains(contentType, "multipart/form-data") {
		if err := req.ParseMultipartForm(64 << 20); err != nil {
			return nil, nil, "", "", fmt.Errorf("parse multipart form: %w", err)
		}
		for k, values := range req.MultipartForm.Value {
			if len(values) > 0 {
				payload[k] = values[0]
			}
		}
		file, header, err := req.FormFile("file")
		if err != nil {
			return nil, nil, "", "", fmt.Errorf("missing file field")
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, nil, "", "", fmt.Errorf("read file: %w", err)
		}
		mime := header.Header.Get("Content-Type")
		if mime == "" {
			mime = "application/octet-stream"
		}
		return payload, data, header.Filename, mime, nil
	}
	raw, err := readJSONBody(req, false)
	if err != nil {
		return nil, nil, "", "", err
	}
	fileURL, _ := raw["file"].(string)
	data, mime, err := decodeDataURL(fileURL)
	if err != nil {
		return nil, nil, "", "", err
	}
	delete(raw, "file")
	return raw, data, "audio.bin", mime, nil
}

func decodeDataURL(v string) ([]byte, string, error) {
	if !strings.HasPrefix(v, "data:") {
		return nil, "", fmt.Errorf("file must be a data url")
	}
	parts := strings.SplitN(v, ",", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("invalid data url")
	}
	header := parts[0]
	content := parts[1]
	mime := "application/octet-stream"
	if strings.Contains(header, ";") {
		mime = strings.TrimPrefix(strings.SplitN(header, ";", 2)[0], "data:")
	}
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, "", fmt.Errorf("invalid base64 audio data")
	}
	return data, mime, nil
}

func handleOpenAIAudioRequest(w http.ResponseWriter, req *http.Request, svc CatalogService, transcription bool) {
	payload, data, filename, mimeType, err := parseAudioRequest(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload, status, detail := enforcePayloadPolicy(req, "", payload)
	if status != 0 {
		writeJSONError(w, status, detail)
		return
	}
	var out map[string]any
	if transcription {
		out, err = svc.OpenAIAudioTranscriptions(req.Context(), "", payload, data, filename, mimeType)
	} else {
		out, err = svc.OpenAIAudioTranslations(req.Context(), "", payload, data, filename, mimeType)
	}
	if err != nil {
		writeInvokeError(w, err, "audio invoke failed")
		return
	}
	maybeAccumulateUsage(req, svc, out, 0)
	writeJSON(w, http.StatusOK, out)
}

func handleOpenAIImagesGenerations(w http.ResponseWriter, req *http.Request, svc CatalogService) {
	payload, err := readJSONBody(req, false)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload, status, detail := enforcePayloadPolicy(req, "", payload)
	if status != 0 {
		writeJSONError(w, status, detail)
		return
	}
	out, err := svc.OpenAIImagesGenerations(req.Context(), "", payload)
	if err != nil {
		writeInvokeError(w, err, "image generation invoke failed")
		return
	}
	maybeAccumulateUsage(req, svc, out, 0)
	writeJSON(w, http.StatusOK, out)
}

func handleOpenAIVideosGenerations(w http.ResponseWriter, req *http.Request, svc CatalogService) {
	payload, err := readJSONBody(req, false)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload, status, detail := enforcePayloadPolicy(req, "", payload)
	if status != 0 {
		writeJSONError(w, status, detail)
		return
	}
	jobID := fmt.Sprintf("vidgen-%d", time.Now().UnixNano())
	item := map[string]any{
		"id":     jobID,
		"object": "video.generation",
		"status": "queued",
	}
	videoJobStoreSet(jobID, item)
	go func() {
		out, invokeErr := svc.OpenAIVideosGenerations(context.Background(), "", payload)
		videoJobsMu.Lock()
		defer videoJobsMu.Unlock()
		entry := videoJobs[jobID]
		if invokeErr != nil {
			entry["status"] = "failed"
			entry["error"] = invokeErr.Error()
		} else {
			entry["status"] = "completed"
			entry["result"] = out
		}
		videoJobs[jobID] = entry
	}()
	writeJSON(w, http.StatusAccepted, item)
}

func newMultipartAudioBody(model string, fileData []byte, filename string, mimeType string, payload map[string]any) ([]byte, string, error) {
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	if err := writer.WriteField("model", model); err != nil {
		return nil, "", err
	}
	for k, v := range payload {
		if k == "model" || v == nil {
			continue
		}
		if err := writer.WriteField(k, fmt.Sprintf("%v", v)); err != nil {
			return nil, "", err
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, "", err
	}
	if mimeType != "" {
		_ = writer.WriteField("mime_type", mimeType)
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}
