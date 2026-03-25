package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rinbarpen/llm-router/src/schemas"
	"github.com/rinbarpen/llm-router/src/services"
)

type fakeCatalogService struct {
	providers []schemas.Provider
	models    []schemas.Model
	apiKeys   []schemas.APIKey
	chatResp  map[string]any
	invokes   []schemas.MonitorInvocation
}

func (f *fakeCatalogService) ListProviders(_ context.Context) ([]schemas.Provider, error) {
	return f.providers, nil
}

func (f *fakeCatalogService) GetProviderByName(_ context.Context, name string) (schemas.Provider, error) {
	for _, p := range f.providers {
		if p.Name == name {
			return p, nil
		}
	}
	return schemas.Provider{}, services.ErrNotFound
}

func (f *fakeCatalogService) CreateProvider(_ context.Context, in schemas.ProviderCreate) (schemas.Provider, error) {
	p := schemas.Provider{ID: int64(len(f.providers) + 1), Name: in.Name, Type: in.Type, IsActive: true, BaseURL: in.BaseURL, APIKey: in.APIKey, Settings: in.Settings}
	f.providers = append(f.providers, p)
	return p, nil
}

func (f *fakeCatalogService) UpdateProvider(_ context.Context, name string, in schemas.ProviderUpdate) (schemas.Provider, error) {
	for i := range f.providers {
		if f.providers[i].Name == name {
			if in.BaseURL != nil {
				f.providers[i].BaseURL = in.BaseURL
			}
			if in.APIKey != nil {
				f.providers[i].APIKey = in.APIKey
			}
			if in.IsActive != nil {
				f.providers[i].IsActive = *in.IsActive
			}
			if in.Settings != nil {
				f.providers[i].Settings = in.Settings
			}
			return f.providers[i], nil
		}
	}
	return schemas.Provider{}, services.ErrNotFound
}

func (f *fakeCatalogService) ListModels(_ context.Context) ([]schemas.Model, error) {
	return f.models, nil
}

func (f *fakeCatalogService) CreateModel(_ context.Context, in schemas.ModelCreate) (schemas.Model, error) {
	m := schemas.Model{ID: int64(len(f.models) + 1), ProviderName: in.ProviderName, Name: in.Name, IsActive: true, RemoteIdentifier: in.RemoteIdentifier}
	f.models = append(f.models, m)
	return m, nil
}

func (f *fakeCatalogService) GetModelByProviderAndName(_ context.Context, providerName string, modelName string) (schemas.Model, error) {
	for _, m := range f.models {
		if m.ProviderName == providerName && m.Name == modelName {
			return m, nil
		}
	}
	return schemas.Model{}, services.ErrNotFound
}

func (f *fakeCatalogService) UpdateModel(_ context.Context, providerName string, modelName string, in schemas.ModelUpdate) (schemas.Model, error) {
	for i := range f.models {
		if f.models[i].ProviderName == providerName && f.models[i].Name == modelName {
			if in.DisplayName != nil {
				f.models[i].DisplayName = in.DisplayName
			}
			if in.Description != nil {
				f.models[i].Description = in.Description
			}
			if in.IsActive != nil {
				f.models[i].IsActive = *in.IsActive
			}
			if in.RemoteIdentifier != nil {
				f.models[i].RemoteIdentifier = in.RemoteIdentifier
			}
			if in.DefaultParams != nil {
				f.models[i].DefaultParams = in.DefaultParams
			}
			if in.Config != nil {
				f.models[i].Config = in.Config
			}
			if in.DownloadURI != nil {
				f.models[i].DownloadURI = in.DownloadURI
			}
			if in.LocalPath != nil {
				f.models[i].LocalPath = in.LocalPath
			}
			return f.models[i], nil
		}
	}
	return schemas.Model{}, services.ErrNotFound
}

func (f *fakeCatalogService) ListModelsByProvider(_ context.Context, providerName string) ([]schemas.Model, error) {
	out := make([]schemas.Model, 0)
	for _, model := range f.models {
		if model.ProviderName == providerName {
			out = append(out, model)
		}
	}
	return out, nil
}

func (f *fakeCatalogService) ListAPIKeys(_ context.Context, includeInactive bool) ([]schemas.APIKey, error) {
	if includeInactive {
		return f.apiKeys, nil
	}
	out := make([]schemas.APIKey, 0)
	for _, item := range f.apiKeys {
		if item.IsActive {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeCatalogService) CreateAPIKey(_ context.Context, in schemas.APIKeyCreate) (schemas.APIKey, error) {
	key := in.Key
	if key == nil {
		v := "generated-key"
		key = &v
	}
	item := schemas.APIKey{
		ID:               int64(len(f.apiKeys) + 1),
		Key:              key,
		Name:             in.Name,
		IsActive:         true,
		AllowedModels:    in.AllowedModels,
		AllowedProviders: in.AllowedProviders,
		ParameterLimits:  in.ParameterLimits,
	}
	f.apiKeys = append(f.apiKeys, item)
	return item, nil
}

func (f *fakeCatalogService) GetAPIKey(_ context.Context, id int64) (schemas.APIKey, error) {
	for _, item := range f.apiKeys {
		if item.ID == id {
			return item, nil
		}
	}
	return schemas.APIKey{}, services.ErrNotFound
}

func (f *fakeCatalogService) UpdateAPIKey(_ context.Context, id int64, in schemas.APIKeyUpdate) (schemas.APIKey, error) {
	for i := range f.apiKeys {
		if f.apiKeys[i].ID == id {
			if in.Name != nil {
				f.apiKeys[i].Name = in.Name
			}
			if in.IsActive != nil {
				f.apiKeys[i].IsActive = *in.IsActive
			}
			return f.apiKeys[i], nil
		}
	}
	return schemas.APIKey{}, services.ErrNotFound
}

func (f *fakeCatalogService) DeleteAPIKey(_ context.Context, id int64) error {
	for i := range f.apiKeys {
		if f.apiKeys[i].ID == id {
			f.apiKeys[i].IsActive = false
			return nil
		}
	}
	return services.ErrNotFound
}

func (f *fakeCatalogService) ValidateAPIKey(_ context.Context, key string) (schemas.APIKey, error) {
	for _, item := range f.apiKeys {
		if item.Key != nil && *item.Key == key && item.IsActive {
			return item, nil
		}
	}
	return schemas.APIKey{}, services.ErrNotFound
}

func (f *fakeCatalogService) OpenAIChatCompletions(_ context.Context, providerHint string, payload map[string]any) (map[string]any, error) {
	if providerHint != "" {
		payload["provider_hint"] = providerHint
	}
	if f.chatResp != nil {
		return f.chatResp, nil
	}
	return map[string]any{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"created": 1,
		"model":   payload["model"],
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "ok",
				},
				"finish_reason": "stop",
			},
		},
	}, nil
}

func (f *fakeCatalogService) OpenAIEmbeddings(_ context.Context, _ string, payload map[string]any) (map[string]any, error) {
	model, _ := payload["model"].(string)
	return map[string]any{
		"object": "list",
		"data":   []map[string]any{{"object": "embedding", "embedding": []float64{0.1, 0.2}, "index": 0}},
		"model":  model,
	}, nil
}

func (f *fakeCatalogService) OpenAIResponses(_ context.Context, _ string, payload map[string]any) (map[string]any, error) {
	model, _ := payload["model"].(string)
	return map[string]any{
		"id":          "resp_123",
		"object":      "response",
		"status":      "completed",
		"model":       model,
		"output_text": "resp-ok",
	}, nil
}

func (f *fakeCatalogService) OpenAIAudioSpeech(_ context.Context, _ string, _ map[string]any) ([]byte, string, error) {
	return []byte("FAKEAUDIO"), "audio/mpeg", nil
}

func (f *fakeCatalogService) OpenAIAudioTranscriptions(_ context.Context, _ string, _ map[string]any, _ []byte, _ string, _ string) (map[string]any, error) {
	return map[string]any{"text": "transcribed"}, nil
}

func (f *fakeCatalogService) OpenAIAudioTranslations(_ context.Context, _ string, _ map[string]any, _ []byte, _ string, _ string) (map[string]any, error) {
	return map[string]any{"text": "translated"}, nil
}

func (f *fakeCatalogService) OpenAIImagesGenerations(_ context.Context, _ string, _ map[string]any) (map[string]any, error) {
	return map[string]any{"created": 1, "data": []map[string]any{{"url": "https://example.com/image.png"}}}, nil
}

func (f *fakeCatalogService) OpenAIVideosGenerations(_ context.Context, _ string, _ map[string]any) (map[string]any, error) {
	return map[string]any{"created": 1, "data": []map[string]any{{"url": "https://example.com/video.mp4"}}}, nil
}

func (f *fakeCatalogService) OpenAIChatCompletionsStream(_ context.Context, _ string, payload map[string]any) (*services.StreamResponse, error) {
	model, _ := payload["model"].(string)
	if model == "" {
		model = "unknown"
	}
	chunk := `data: {"id":"chatcmpl-stream","object":"chat.completion.chunk","model":"` + model + `","choices":[{"index":0,"delta":{"content":"hello"}}]}`
	body := chunk + "\n\n" + "data: [DONE]\n\n"
	return &services.StreamResponse{
		Body:        io.NopCloser(strings.NewReader(body)),
		ContentType: "text/event-stream",
	}, nil
}

func (f *fakeCatalogService) GeminiGenerateContent(_ context.Context, modelName string, payload map[string]any) (map[string]any, error) {
	if f.chatResp != nil {
		return map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"role": "model",
						"parts": []map[string]any{
							{"text": "gemini-ok"},
						},
					},
					"finishReason": "STOP",
					"index":        0,
				},
			},
			"modelVersion": modelName,
		}, nil
	}
	_ = payload
	return map[string]any{
		"candidates": []map[string]any{
			{
				"content": map[string]any{
					"role": "model",
					"parts": []map[string]any{
						{"text": "gemini-ok"},
					},
				},
				"finishReason": "STOP",
				"index":        0,
			},
		},
		"modelVersion": modelName,
	}, nil
}

func (f *fakeCatalogService) GeminiStreamGenerateContent(_ context.Context, modelName string, payload map[string]any) (*services.StreamResponse, error) {
	_ = payload
	chunk := `data: {"id":"chatcmpl-gemini-stream","object":"chat.completion.chunk","model":"` + modelName + `","choices":[{"index":0,"delta":{"content":"gemini-stream-ok"}}]}`
	body := chunk + "\n\n" + "data: [DONE]\n\n"
	return &services.StreamResponse{
		Body:        io.NopCloser(strings.NewReader(body)),
		ContentType: "text/event-stream",
	}, nil
}

func (f *fakeCatalogService) ClaudeMessages(_ context.Context, payload map[string]any) (map[string]any, error) {
	model, _ := payload["model"].(string)
	if model == "" {
		model = "claude-4.5-sonnet"
	}
	return map[string]any{
		"id":          "msg_123",
		"type":        "message",
		"role":        "assistant",
		"model":       model,
		"stop_reason": "end_turn",
		"content": []map[string]any{
			{"type": "text", "text": "claude-ok"},
		},
	}, nil
}

func (f *fakeCatalogService) ClaudeCountTokens(_ context.Context, payload map[string]any) (map[string]any, error) {
	_ = payload
	return map[string]any{"input_tokens": 10}, nil
}

func (f *fakeCatalogService) ClaudeCreateMessageBatch(_ context.Context, payload map[string]any) (map[string]any, error) {
	_ = payload
	return map[string]any{"id": "msgbatch_123", "status": "in_progress"}, nil
}

func (f *fakeCatalogService) ClaudeGetMessageBatch(_ context.Context, batchID string) (map[string]any, error) {
	return map[string]any{"id": batchID, "status": "in_progress"}, nil
}

func (f *fakeCatalogService) ClaudeCancelMessageBatch(_ context.Context, batchID string) (map[string]any, error) {
	return map[string]any{"id": batchID, "status": "cancelled"}, nil
}

func (f *fakeCatalogService) ListInvocations(_ context.Context, limit int, offset int) ([]schemas.MonitorInvocation, error) {
	if offset >= len(f.invokes) {
		return []schemas.MonitorInvocation{}, nil
	}
	end := offset + limit
	if end > len(f.invokes) {
		end = len(f.invokes)
	}
	return f.invokes[offset:end], nil
}

func (f *fakeCatalogService) GetInvocation(_ context.Context, id int64) (schemas.MonitorInvocation, error) {
	for _, item := range f.invokes {
		if item.ID == id {
			return item, nil
		}
	}
	return schemas.MonitorInvocation{}, services.ErrNotFound
}

func (f *fakeCatalogService) GetInvocationStatistics(_ context.Context) (map[string]any, error) {
	total := int64(len(f.invokes))
	success := int64(0)
	var totalCost float64
	var totalTokens int64
	for _, item := range f.invokes {
		if item.Status == "success" {
			success++
		}
		if item.Cost != nil {
			totalCost += *item.Cost
		}
		if item.TotalTokens != nil {
			totalTokens += *item.TotalTokens
		}
	}
	return map[string]any{
		"total_invocations": total,
		"success_count":     success,
		"error_count":       total - success,
		"total_cost":        totalCost,
		"total_tokens":      totalTokens,
	}, nil
}

func (f *fakeCatalogService) GetMonitorTimeSeries(_ context.Context, granularity string, _ int) (schemas.TimeSeriesResponse, error) {
	switch granularity {
	case "hour", "day", "week", "month":
		return schemas.TimeSeriesResponse{Granularity: granularity, Data: []schemas.TimeSeriesDataPoint{}}, nil
	default:
		return schemas.TimeSeriesResponse{}, fmt.Errorf("unsupported granularity: %s", granularity)
	}
}

func (f *fakeCatalogService) GetMonitorGroupedTimeSeries(_ context.Context, groupBy, granularity string, _ int) (schemas.GroupedTimeSeriesResponse, error) {
	switch groupBy {
	case "model", "provider":
	default:
		return schemas.GroupedTimeSeriesResponse{}, fmt.Errorf("unsupported group_by: %s", groupBy)
	}
	switch granularity {
	case "hour", "day", "week", "month":
	default:
		return schemas.GroupedTimeSeriesResponse{}, fmt.Errorf("unsupported granularity: %s", granularity)
	}
	return schemas.GroupedTimeSeriesResponse{
		Granularity: granularity,
		GroupBy:     groupBy,
		Data:        []schemas.GroupedTimeSeriesDataPoint{},
	}, nil
}

func (f *fakeCatalogService) ExportInvocationsCSV(_ context.Context, limit int, offset int) ([]byte, error) {
	items, _ := f.ListInvocations(context.Background(), limit, offset)
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"id", "provider_name", "model_name", "status"})
	for _, item := range items {
		_ = w.Write([]string{
			int64ToString(item.ID),
			item.ProviderName,
			item.ModelName,
			item.Status,
		})
	}
	w.Flush()
	return []byte(b.String()), nil
}

func (f *fakeCatalogService) ExportMonitorDatabaseSQLite(_ context.Context) ([]byte, error) {
	return []byte("SQLite format 3\x00fake"), nil
}

func (f *fakeCatalogService) GetLatestPricing(_ context.Context) ([]map[string]any, error) {
	return []map[string]any{
		{"model": "gpt-4o", "provider": "openai", "avg_cost": 0.02},
	}, nil
}

func (f *fakeCatalogService) GetPricingSuggestions(_ context.Context) ([]map[string]any, error) {
	return []map[string]any{
		{"model": "gpt-4o-mini", "provider": "openai", "avg_cost": 0.005, "reason": "lower observed average cost"},
	}, nil
}

func (f *fakeCatalogService) SyncModelPricing(_ context.Context, modelID int64) (map[string]any, error) {
	return map[string]any{"success": true, "model_id": modelID}, nil
}

func (f *fakeCatalogService) SyncAllPricing(_ context.Context) (map[string]any, error) {
	return map[string]any{"success": true}, nil
}

func (f *fakeCatalogService) ListLoginRecords(_ context.Context, limit int, offset int) ([]services.LoginRecord, int, error) {
	items := []services.LoginRecord{
		{ID: 1},
	}
	return items, len(items), nil
}

func (f *fakeCatalogService) OAuthAuthorizeURL(_ context.Context, providerType string, providerName string, _ string, _ string) (string, string, error) {
	return "https://example.com/auth/" + providerType + "?provider=" + providerName, "state-123", nil
}

func (f *fakeCatalogService) OAuthHandleCallback(_ context.Context, _ string, _ string, _ string) (string, error) {
	return "https://example.com/monitor?oauth=success", nil
}

func (f *fakeCatalogService) OAuthHasCredential(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (f *fakeCatalogService) OAuthRevokeCredential(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (f *fakeCatalogService) SyncRouterTOML(_ context.Context, _ string) error {
	return nil
}

func TestProvidersEndpoints(t *testing.T) {
	svc := &fakeCatalogService{}
	r := NewRouter(svc)

	createBody := []byte(`{"name":"p1","type":"openai"}`)
	req := httptest.NewRequest(http.MethodPost, "/providers", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("create provider status=%d want=201 body=%s", rr.Code, rr.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/providers", nil)
	listRR := httptest.NewRecorder()
	r.ServeHTTP(listRR, listReq)

	if listRR.Code != http.StatusOK {
		t.Fatalf("list provider status=%d want=200 body=%s", listRR.Code, listRR.Body.String())
	}

	var providers []map[string]any
	if err := json.Unmarshal(listRR.Body.Bytes(), &providers); err != nil {
		t.Fatalf("decode providers: %v", err)
	}
	if len(providers) != 1 || providers[0]["name"] != "p1" {
		t.Fatalf("unexpected providers payload: %+v", providers)
	}
}

func TestModelsAndOpenAIModelsEndpoint(t *testing.T) {
	svc := &fakeCatalogService{
		models: []schemas.Model{{ID: 1, ProviderName: "p1", Name: "gpt-4o", IsActive: true}},
	}
	r := NewRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("v1/models status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode /v1/models: %v", err)
	}
	if payload["object"] != "list" {
		t.Fatalf("unexpected object field: %+v", payload)
	}
}

func TestAPIPrefixMirrorsCoreEndpoints(t *testing.T) {
	svc := &fakeCatalogService{}
	r := NewRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("/api/health status=%d want=200", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("/api/providers status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
}

func TestProviderModelEndpoints(t *testing.T) {
	svc := &fakeCatalogService{
		models: []schemas.Model{
			{ID: 1, ProviderName: "p1", Name: "gpt-4o", IsActive: true},
			{ID: 2, ProviderName: "p1", Name: "gpt-4o-mini", IsActive: true},
			{ID: 3, ProviderName: "p2", Name: "gemini-2.5-pro", IsActive: true},
		},
	}
	r := NewRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/models/p1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/models/p1 status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/providers/p1/supported-models", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("supported-models status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode supported-models payload: %v", err)
	}
	models, ok := payload["models"].([]any)
	if !ok || len(models) != 2 {
		t.Fatalf("unexpected supported-models payload: %+v", payload)
	}
}

func TestAPIKeyAndAuthEndpoints(t *testing.T) {
	key := "k-1"
	svc := &fakeCatalogService{
		apiKeys: []schemas.APIKey{
			{ID: 1, Key: &key, Name: ptr("k1"), IsActive: true},
		},
	}
	r := NewRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api-keys", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list api keys status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	loginBody := []byte(`{"api_key":"k-1"}`)
	req = httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode login payload: %v", err)
	}
	token, ok := payload["session_token"].(string)
	if !ok || token == "" {
		t.Fatalf("session_token not found: %+v", payload)
	}

	req = httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("logout status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
}

func ptr(v string) *string { return &v }

func TestProviderAndModelUpdateEndpoints(t *testing.T) {
	baseURL := "https://old.example.com"
	newBaseURL := "https://new.example.com"
	display := "old"
	newDisplay := "new-display"
	svc := &fakeCatalogService{
		providers: []schemas.Provider{
			{ID: 1, Name: "p1", Type: "openai", IsActive: true, BaseURL: &baseURL},
		},
		models: []schemas.Model{
			{ID: 1, ProviderName: "p1", Name: "gpt-4o", IsActive: true, DisplayName: &display},
		},
	}
	r := NewRouter(svc)

	updateProviderBody := []byte(`{"base_url":"https://new.example.com"}`)
	req := httptest.NewRequest(http.MethodPatch, "/providers/p1", bytes.NewReader(updateProviderBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch provider status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	var providerPayload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &providerPayload)
	if providerPayload["base_url"] != newBaseURL {
		t.Fatalf("provider update not applied: %+v", providerPayload)
	}

	req = httptest.NewRequest(http.MethodGet, "/models/p1/gpt-4o", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get model status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	updateModelBody := []byte(`{"display_name":"new-display"}`)
	req = httptest.NewRequest(http.MethodPatch, "/models/p1/gpt-4o", bytes.NewReader(updateModelBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch model status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	var modelPayload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &modelPayload)
	if modelPayload["display_name"] != newDisplay {
		t.Fatalf("model update not applied: %+v", modelPayload)
	}
}

func TestOpenAIChatCompletionsEndpoints(t *testing.T) {
	svc := &fakeCatalogService{}
	r := NewRouter(svc)

	body := []byte(`{"model":"p1/gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/chat/completions status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode /v1/chat/completions: %v", err)
	}
	if payload["object"] != "chat.completion" {
		t.Fatalf("unexpected chat completion payload: %+v", payload)
	}

	body = []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	req = httptest.NewRequest(http.MethodPost, "/p1/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/p1/v1/chat/completions status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	streamBody := []byte(`{"model":"p1/gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(streamBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("stream /v1/chat/completions status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "data: [DONE]") {
		t.Fatalf("stream /v1/chat/completions missing [DONE], body=%s", rr.Body.String())
	}
}

func TestRouteDecisionAndInvokeEndpoints(t *testing.T) {
	baseURL := "https://example.com/v1"
	svc := &fakeCatalogService{
		providers: []schemas.Provider{
			{ID: 1, Name: "openrouter", Type: "openai", IsActive: true, BaseURL: &baseURL},
		},
		models: []schemas.Model{
			{ID: 1, ProviderName: "openrouter", Name: "gpt-4o", IsActive: true},
		},
		chatResp: map[string]any{
			"id":      "chatcmpl-route",
			"object":  "chat.completion",
			"created": 1,
			"model":   "openrouter/gpt-4o",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "hello from route invoke",
					},
					"finish_reason": "stop",
				},
			},
		},
	}
	r := NewRouter(svc)

	routeBody := []byte(`{"model_hint":"openrouter/gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/route", bytes.NewReader(routeBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/route status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	var routePayload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &routePayload); err != nil {
		t.Fatalf("decode /route payload: %v", err)
	}
	if routePayload["provider"] != "openrouter" {
		t.Fatalf("unexpected route provider: %+v", routePayload)
	}

	invokeBody := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	req = httptest.NewRequest(http.MethodPost, "/route/invoke", bytes.NewReader(invokeBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/route/invoke status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	var invokePayload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &invokePayload); err != nil {
		t.Fatalf("decode /route/invoke payload: %v", err)
	}
	if invokePayload["object"] != "chat.completion" {
		t.Fatalf("unexpected /route/invoke payload: %+v", invokePayload)
	}
}

func TestGeminiGenerateContentEndpoints(t *testing.T) {
	svc := &fakeCatalogService{}
	r := NewRouter(svc)

	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-pro:generateContent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("gemini generateContent status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-pro:streamGenerateContent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("gemini streamGenerateContent status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "gemini-stream-ok") {
		t.Fatalf("unexpected gemini stream body: %s", rr.Body.String())
	}
}

func TestClaudeNativeEndpoints(t *testing.T) {
	svc := &fakeCatalogService{}
	r := NewRouter(svc)

	body := []byte(`{"model":"claude-4.5-sonnet","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/messages status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/messages/count_tokens status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/messages/batches", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/messages/batches status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/messages/batches/msgbatch_123", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/messages/batches/{id} status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/messages/batches/msgbatch_123/cancel", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/messages/batches/{id}/cancel status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
}

func TestOpenAIExtendedEndpoints(t *testing.T) {
	svc := &fakeCatalogService{
		apiKeys: []schemas.APIKey{
			{ID: 1, IsActive: true, Key: ptrString("fixture-key")},
		},
		models: []schemas.Model{
			{ID: 1, ProviderName: "openai", Name: "gpt-4o", IsActive: true},
		},
	}
	r := NewRouter(svc)

	respReq := []byte(`{"model":"openai/gpt-4o","input":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(respReq))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/responses status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	embReq := []byte(`{"model":"openai/gpt-4o","input":"hello"}`)
	req = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(embReq))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/embeddings status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	audioReq := []byte(`{"model":"openai/gpt-4o","input":"hello","voice":"alloy"}`)
	req = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewReader(audioReq))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/audio/speech status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	audioDataURL := "data:audio/wav;base64,UklGRkQAAABXQVZFZm10IBAAAAABAAEA"
	asrReq := []byte(`{"model":"openai/gpt-4o","file":"` + audioDataURL + `"}`)
	req = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", bytes.NewReader(asrReq))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/audio/transcriptions status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	imgReq := []byte(`{"model":"openai/gpt-4o","prompt":"draw"}`)
	req = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(imgReq))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/images/generations status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	videoReq := []byte(`{"model":"openai/gpt-4o","prompt":"video"}`)
	req = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewReader(videoReq))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("/v1/videos/generations status=%d want=202 body=%s", rr.Code, rr.Body.String())
	}
	var job map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode video generation job: %v", err)
	}
	jobID, _ := job["id"].(string)
	if strings.TrimSpace(jobID) == "" {
		t.Fatalf("missing video generation job id")
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/videos/generations/"+jobID, nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/v1/videos/generations/{id} status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthBindModelOAuthAndPricingSyncEndpoints(t *testing.T) {
	svc := &fakeCatalogService{
		apiKeys: []schemas.APIKey{
			{ID: 1, IsActive: true, Key: ptrString("fixture-key")},
		},
		models: []schemas.Model{
			{ID: 1, ProviderName: "openai", Name: "gpt-4o", IsActive: true},
		},
	}
	r := NewRouter(svc)

	loginReq := []byte(`{"api_key":"fixture-key"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginReq))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/auth/login status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	var loginPayload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode /auth/login payload: %v", err)
	}
	sessionToken, _ := loginPayload["session_token"].(string)
	if strings.TrimSpace(sessionToken) == "" {
		t.Fatalf("/auth/login missing session_token")
	}

	bindReq := []byte(`{"provider_name":"openai","model_name":"gpt-4o","binding_type":"strong"}`)
	req = httptest.NewRequest(http.MethodPost, "/auth/bind-model", bytes.NewReader(bindReq))
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/auth/bind-model status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/auth/oauth/openrouter/authorize?provider_name=openrouter-main&callback_url=http://example.com", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/auth/oauth/{provider}/authorize status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/auth/oauth/openrouter/status?provider_name=openrouter-main", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/auth/oauth/{provider}/status status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/auth/oauth/openrouter/revoke", strings.NewReader(`{"provider_name":"openrouter-main"}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/auth/oauth/{provider}/revoke status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/login-records?limit=10&offset=0", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/login-records status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/pricing/sync/1", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/pricing/sync/{model_id} status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/pricing/sync-all", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/pricing/sync-all status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddlewareProtectedEndpoints(t *testing.T) {
	key := "auth-fixture-key"
	svc := &fakeCatalogService{
		apiKeys: []schemas.APIKey{
			{ID: 1, Key: &key, IsActive: true},
		},
	}
	r := NewRouterWithOptions(svc, RouterOptions{
		RequireAuth:      true,
		AllowLocalNoAuth: false,
	})

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	req.RemoteAddr = "203.0.113.8:1234"
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no auth status=%d want=401 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/models", nil)
	req.RemoteAddr = "203.0.113.8:1234"
	req.Header.Set("Authorization", "Bearer "+key)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("valid api key status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "203.0.113.8:1234"
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("public health should bypass auth, status=%d", rr.Code)
	}
}

func TestMonitorEndpoints(t *testing.T) {
	cost := 0.02
	totalTokens := int64(120)
	svc := &fakeCatalogService{
		invokes: []schemas.MonitorInvocation{
			{ID: 1, ModelName: "gpt-4o", ProviderName: "openai", Status: "success", Cost: &cost, TotalTokens: &totalTokens},
		},
	}
	r := NewRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/monitor/invocations?limit=10&offset=0", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/invocations status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/invocations/1", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/invocations/1 status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/statistics", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/statistics status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/time-series?granularity=day&time_range_hours=24", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/time-series status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	var ts schemas.TimeSeriesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &ts); err != nil {
		t.Fatalf("decode time-series: %v", err)
	}
	if ts.Granularity != "day" || ts.Data == nil {
		t.Fatalf("unexpected time-series payload: %+v", ts)
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/time-series?granularity=invalid&time_range_hours=24", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("/monitor/time-series invalid granularity status=%d want=400 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/time-series?granularity=day&time_range_hours=0", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("/monitor/time-series time_range_hours=0 status=%d want=400 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/grouped-time-series?group_by=model&granularity=day&time_range_hours=24", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/grouped-time-series status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	var gts schemas.GroupedTimeSeriesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &gts); err != nil {
		t.Fatalf("decode grouped time-series: %v", err)
	}
	if gts.Granularity != "day" || gts.GroupBy != "model" || gts.Data == nil {
		t.Fatalf("unexpected grouped time-series payload: %+v", gts)
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/grouped-time-series?group_by=bad&granularity=day&time_range_hours=24", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("/monitor/grouped-time-series invalid group_by status=%d want=400 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/export/json", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/export/json status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/export/excel", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/export/excel status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/database", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/database status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/x-sqlite3" {
		t.Fatalf("/monitor/database content-type=%q want=application/x-sqlite3", got)
	}
	if rr.Body.Len() == 0 {
		t.Fatalf("/monitor/database body should not be empty")
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/database?format=zip", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/database?format=zip status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "application/zip") {
		t.Fatalf("/monitor/database?format=zip content-type=%q want contains application/zip", got)
	}
	zipReader, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
	if err != nil {
		t.Fatalf("open monitor export zip: %v", err)
	}
	names := map[string]bool{}
	for _, f := range zipReader.File {
		names[f.Name] = true
	}
	for _, required := range []string{"monitor_invocations.csv", "monitor_invocations.json", "metadata.json"} {
		if !names[required] {
			t.Fatalf("monitor export zip missing %s; entries=%v", required, names)
		}
	}
}

func TestPricingAndConfigSyncEndpoints(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "router.toml")
	if err := os.WriteFile(cfgPath, []byte("[[providers]]\nname = \"p1\"\ntype = \"openai\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := &fakeCatalogService{}
	r := NewRouterWithOptions(svc, RouterOptions{ModelConfigHintPath: cfgPath})

	req := httptest.NewRequest(http.MethodGet, "/pricing/latest", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/pricing/latest status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/pricing/suggestions", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/pricing/suggestions status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/config/sync", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/config/sync status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

func ptrString(v string) *string {
	return &v
}
