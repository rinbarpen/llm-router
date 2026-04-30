package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rinbarpen/llm-router/src/logging"
	"github.com/rinbarpen/llm-router/src/schemas"
	"github.com/rinbarpen/llm-router/src/services"
)

type fakeCatalogService struct {
	providers           []schemas.Provider
	models              []schemas.Model
	supportedModels     map[string][]string
	supportedModelsErr  error
	remoteModels        map[string][]services.RemoteProviderModel
	remoteModelsErr     error
	modelUpdateRuns     []services.ModelUpdateRun
	apiKeys             []schemas.APIKey
	chatResp            map[string]any
	ttsPlugins          []map[string]any
	ttsVoices           map[string][]map[string]any
	invokes             []schemas.MonitorInvocation
	streamOpenFailCount int
	usageCalls          int
	lastUsageAPIKeyID   int64
	lastUsageTokens     int64
	lastUsageCost       float64
	consoleLogin        map[string]any
	consoleProfile      map[string]any
	walletSummary       map[string]any
	rechargeOrders      []map[string]any
	chatErr             error
	onChat              func(context.Context, string, map[string]any)
	walletCheckErr      error
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

func (f *fakeCatalogService) ListProviderSupportedModels(_ context.Context, providerName string) ([]string, error) {
	if f.supportedModelsErr != nil {
		return nil, f.supportedModelsErr
	}
	if f.supportedModels != nil {
		if models, ok := f.supportedModels[providerName]; ok {
			out := make([]string, len(models))
			copy(out, models)
			return out, nil
		}
	}
	for _, p := range f.providers {
		if p.Name == providerName {
			return nil, nil
		}
	}
	if f.supportedModels != nil {
		return nil, services.ErrNotFound
	}
	out := make([]string, 0)
	for _, model := range f.models {
		if model.ProviderName == providerName {
			out = append(out, model.Name)
		}
	}
	return out, nil
}

func (f *fakeCatalogService) ListProviderRemoteModels(_ context.Context, providerName string, refresh bool) ([]services.RemoteProviderModel, error) {
	_ = refresh
	if f.remoteModelsErr != nil {
		return nil, f.remoteModelsErr
	}
	if f.remoteModels != nil {
		if models, ok := f.remoteModels[providerName]; ok {
			out := make([]services.RemoteProviderModel, len(models))
			copy(out, models)
			return out, nil
		}
	}
	for _, p := range f.providers {
		if p.Name == providerName {
			return nil, nil
		}
	}
	return nil, services.ErrNotFound
}

func (f *fakeCatalogService) SyncProviderModelsFromRemote(_ context.Context, providerName string, defaultNewModelActive bool) (services.ModelUpdateRun, error) {
	_ = defaultNewModelActive
	for _, p := range f.providers {
		if p.Name == providerName {
			run := services.ModelUpdateRun{
				ProviderName: providerName,
				Added:        []string{"remote-a"},
				Updated:      []string{"remote-b"},
				Disabled:     []string{"remote-old"},
			}
			f.modelUpdateRuns = append(f.modelUpdateRuns, run)
			return run, nil
		}
	}
	return services.ModelUpdateRun{}, services.ErrNotFound
}

func (f *fakeCatalogService) SyncAllProviderModelsFromRemote(_ context.Context, opts services.ProviderModelSyncOptions) (services.ModelUpdateResult, error) {
	_ = opts
	result := services.ModelUpdateResult{}
	for _, p := range f.providers {
		run := services.ModelUpdateRun{ProviderName: p.Name, Added: []string{p.Name + "-model"}}
		result.ProviderRuns = append(result.ProviderRuns, run)
		f.modelUpdateRuns = append(f.modelUpdateRuns, run)
	}
	return result, nil
}

func (f *fakeCatalogService) ListModelUpdateRuns(_ context.Context) ([]services.ModelUpdateRun, error) {
	out := make([]services.ModelUpdateRun, len(f.modelUpdateRuns))
	copy(out, f.modelUpdateRuns)
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

func (f *fakeCatalogService) OpenAIChatCompletions(ctx context.Context, providerHint string, payload map[string]any) (map[string]any, error) {
	if f.onChat != nil {
		f.onChat(ctx, providerHint, payload)
	}
	if providerHint != "" {
		payload["provider_hint"] = providerHint
	}
	if f.chatErr != nil {
		return nil, f.chatErr
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

func (f *fakeCatalogService) ListTTSPlugins(_ context.Context) ([]map[string]any, error) {
	return f.ttsPlugins, nil
}

func (f *fakeCatalogService) ListTTSPluginVoices(_ context.Context, pluginName string, modelID string) ([]map[string]any, error) {
	if f.ttsVoices == nil {
		return nil, nil
	}
	return f.ttsVoices[pluginName+"::"+modelID], nil
}

func (f *fakeCatalogService) OpenAIImagesGenerations(_ context.Context, _ string, _ map[string]any) (map[string]any, error) {
	return map[string]any{"created": 1, "data": []map[string]any{{"url": "https://example.com/image.png"}}}, nil
}

func (f *fakeCatalogService) OpenAIVideosGenerations(_ context.Context, _ string, _ map[string]any) (map[string]any, error) {
	return map[string]any{"created": 1, "data": []map[string]any{{"url": "https://example.com/video.mp4"}}}, nil
}

func (f *fakeCatalogService) OpenAIChatCompletionsStream(_ context.Context, _ string, payload map[string]any) (*services.StreamResponse, error) {
	if f.streamOpenFailCount > 0 {
		f.streamOpenFailCount--
		return nil, &services.UpstreamStatusError{StatusCode: http.StatusBadGateway, Detail: "temporary stream open failure"}
	}
	model, _ := payload["model"].(string)
	if model == "" {
		model = "unknown"
	}
	chunk := `data: {"id":"chatcmpl-stream","object":"chat.completion.chunk","model":"` + model + `","choices":[{"index":0,"delta":{"content":"hello"}}]}`
	body := chunk + "\n\n"
	if streamOptions, ok := payload["stream_options"].(map[string]any); ok {
		if includeUsage, ok := streamOptions["include_usage"].(bool); ok && includeUsage {
			body += `data: {"usage":{"total_tokens":42},"cost":0.123}` + "\n\n"
		}
	}
	body += "data: [DONE]\n\n"
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

func TestChatStreamOpenRetry(t *testing.T) {
	origBackoff := streamOpenRetryBackoff
	streamOpenRetryBackoff = 1 * time.Millisecond
	t.Cleanup(func() {
		streamOpenRetryBackoff = origBackoff
	})

	svc := &fakeCatalogService{streamOpenFailCount: 1}
	r := NewRouter(svc)

	body := []byte(`{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("chat stream retry status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "chatcmpl-stream") {
		t.Fatalf("unexpected stream payload: %s", rr.Body.String())
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

func (f *fakeCatalogService) OAuthAuthorizeURL(_ context.Context, providerType string, providerName string, _ string, _ string, _ string, _ bool) (string, string, error) {
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

func (f *fakeCatalogService) ListOAuthAccounts(_ context.Context, providerName string) ([]schemas.OAuthAccount, error) {
	return []schemas.OAuthAccount{
		{
			ID:           1,
			ProviderName: providerName,
			ProviderType: "openrouter",
			AccountName:  "acc-1",
			IsDefault:    true,
			IsActive:     true,
		},
	}, nil
}

func (f *fakeCatalogService) UpdateOAuthAccount(_ context.Context, providerName string, accountID int64, in schemas.OAuthAccountUpdate) (schemas.OAuthAccount, error) {
	out := schemas.OAuthAccount{
		ID:           accountID,
		ProviderName: providerName,
		ProviderType: "openrouter",
		AccountName:  "acc-updated",
		IsDefault:    false,
		IsActive:     true,
	}
	if in.AccountName != nil {
		out.AccountName = *in.AccountName
	}
	if in.IsDefault != nil {
		out.IsDefault = *in.IsDefault
	}
	if in.IsActive != nil {
		out.IsActive = *in.IsActive
	}
	return out, nil
}

func (f *fakeCatalogService) SetDefaultOAuthAccount(_ context.Context, providerName string, accountID int64) (schemas.OAuthAccount, error) {
	return schemas.OAuthAccount{
		ID:           accountID,
		ProviderName: providerName,
		ProviderType: "openrouter",
		AccountName:  "acc-default",
		IsDefault:    true,
		IsActive:     true,
	}, nil
}

func (f *fakeCatalogService) RevokeOAuthAccount(_ context.Context, _ string, _ int64) (bool, error) {
	return true, nil
}

func (f *fakeCatalogService) SyncRouterTOML(_ context.Context, _ string) error {
	return nil
}

func (f *fakeCatalogService) RunSelfCheck(_ context.Context) (map[string]any, error) {
	return map[string]any{
		"overall_status": "ok",
		"checks": map[string]any{
			"database": map[string]any{
				"status": "ok",
			},
		},
	}, nil
}

func (f *fakeCatalogService) GetQuotaDetails(_ context.Context, _ services.QuotaDetailQuery) ([]map[string]any, error) {
	return []map[string]any{
		{"api_key_id": int64(1), "api_key_name": "k1", "provider": "openai", "model": "gpt-4o", "requests": int64(3), "total_tokens": int64(1000)},
	}, nil
}

func (f *fakeCatalogService) ExportQuotaDetailsCSV(_ context.Context, _ services.QuotaDetailQuery) ([]byte, error) {
	return []byte("api_key_id,api_key_name,provider,model,requests,total_tokens,total_cost\n1,k1,openai,gpt-4o,3,1000,0.01\n"), nil
}

func (f *fakeCatalogService) GetBudgetAlerts(_ context.Context) (map[string]any, error) {
	return map[string]any{"alerts": map[string]any{"day": false, "week": false, "month": false}}, nil
}

func (f *fakeCatalogService) UpdateBudgetAlerts(_ context.Context, day, week, month int64) (map[string]any, error) {
	return map[string]any{"thresholds": map[string]any{"day_tokens": day, "week_tokens": week, "month_tokens": month}}, nil
}

func (f *fakeCatalogService) ListAPIKeyPolicyTemplates(_ context.Context, _, _ string) ([]map[string]any, error) {
	return []map[string]any{{"id": int64(1), "name": "default"}}, nil
}

func (f *fakeCatalogService) CreateAPIKeyPolicyTemplate(_ context.Context, name string, _, _ *string, policy map[string]any) (map[string]any, error) {
	return map[string]any{"id": int64(2), "name": name, "policy": policy}, nil
}

func (f *fakeCatalogService) UpdateAPIKeyPolicyTemplate(_ context.Context, id int64, name string, _, _ *string, policy map[string]any) (map[string]any, error) {
	return map[string]any{"id": id, "name": name, "policy": policy}, nil
}

func (f *fakeCatalogService) DeleteAPIKeyPolicyTemplate(_ context.Context, _ int64) error {
	return nil
}

func (f *fakeCatalogService) ApplyAPIKeyPolicyTemplate(_ context.Context, templateID int64, apiKeyIDs []int64) (map[string]any, error) {
	return map[string]any{"template_id": templateID, "updated_keys": len(apiKeyIDs)}, nil
}

func (f *fakeCatalogService) ListAPIKeyPolicyAudit(_ context.Context, _, _ int) ([]map[string]any, error) {
	return []map[string]any{{"id": int64(1), "action": "batch_apply"}}, nil
}

func (f *fakeCatalogService) SyncProviderModelCatalog(_ context.Context, providerName string) (map[string]any, error) {
	return map[string]any{"provider_name": providerName, "count": 1}, nil
}

func (f *fakeCatalogService) ListProviderModelCatalog(_ context.Context, providerName string) ([]map[string]any, error) {
	return []map[string]any{{"provider_name": providerName, "model_name": "gpt-4o"}}, nil
}

func (f *fakeCatalogService) ReconcileProviderModels(_ context.Context, providerName string) (map[string]any, error) {
	return map[string]any{"provider_name": providerName, "missing_in_local": []string{}, "missing_in_catalog": []string{}}, nil
}

func (f *fakeCatalogService) CheckAPIKeyQuota(_ context.Context, _ int64, _ int64) error {
	return nil
}

func (f *fakeCatalogService) AccumulateAPIKeyUsage(_ context.Context, apiKeyID int64, tokens int64, cost float64) error {
	f.usageCalls++
	f.lastUsageAPIKeyID = apiKeyID
	f.lastUsageTokens = tokens
	f.lastUsageCost = cost
	return nil
}

func (f *fakeCatalogService) ConsolePasswordLogin(_ context.Context, email string, password string, remoteAddr string, userAgent string) (schemas.ConsoleSession, error) {
	_ = password
	_ = remoteAddr
	_ = userAgent
	return schemas.ConsoleSession{
		Token: "console-session-token",
		User: schemas.User{
			ID:          7,
			Email:       email,
			DisplayName: "Console User",
			Status:      "active",
			Roles:       []string{"platform_admin"},
		},
	}, nil
}

func (f *fakeCatalogService) GetConsoleSession(_ context.Context, token string) (schemas.ConsoleSession, error) {
	return schemas.ConsoleSession{
		Token: token,
		User: schemas.User{
			ID:          7,
			Email:       "owner@example.com",
			DisplayName: "Console User",
			Status:      "active",
			Roles:       []string{"platform_admin"},
		},
	}, nil
}

func (f *fakeCatalogService) DeleteConsoleSession(_ context.Context, _ string) error {
	return nil
}

func (f *fakeCatalogService) ListConsoleUsers(_ context.Context) ([]schemas.User, error) {
	return []schemas.User{
		{ID: 7, Email: "owner@example.com", DisplayName: "Console User", Status: "active", Roles: []string{"platform_admin"}},
	}, nil
}

func (f *fakeCatalogService) UpdateConsoleUser(_ context.Context, id int64, in schemas.UserUpdate) (schemas.User, error) {
	status := "active"
	if in.Status != nil {
		status = *in.Status
	}
	return schemas.User{ID: id, Email: "owner@example.com", DisplayName: "Console User", Status: status, Roles: []string{"platform_admin"}}, nil
}

func (f *fakeCatalogService) ListConsoleTeams(_ context.Context) ([]schemas.Team, error) {
	return []schemas.Team{
		{ID: 9, Name: "Ops", Slug: "ops", Status: "active"},
	}, nil
}

func (f *fakeCatalogService) CreateConsoleTeam(_ context.Context, in schemas.TeamCreate, _ *int64) (schemas.Team, error) {
	return schemas.Team{ID: 10, Name: in.Name, Slug: in.Slug, Status: "active", Description: in.Description}, nil
}

func (f *fakeCatalogService) ListTeamMembers(_ context.Context, teamID int64) ([]schemas.TeamMember, error) {
	return []schemas.TeamMember{
		{ID: 1, TeamID: teamID, UserID: 7, UserEmail: "owner@example.com", DisplayName: "Console User", Role: "team_owner", Status: "active"},
	}, nil
}

func (f *fakeCatalogService) AddTeamMember(_ context.Context, teamID int64, in schemas.TeamMemberCreate, _ *int64) (schemas.TeamMember, error) {
	role := in.Role
	if role == "" {
		role = "member"
	}
	return schemas.TeamMember{ID: 2, TeamID: teamID, UserID: in.UserID, UserEmail: "member@example.com", DisplayName: "New Member", Role: role, Status: "active"}, nil
}

func (f *fakeCatalogService) UpdateTeamMember(_ context.Context, teamID int64, userID int64, in schemas.TeamMemberUpdate) (schemas.TeamMember, error) {
	role := "member"
	status := "active"
	if in.Role != nil {
		role = *in.Role
	}
	if in.Status != nil {
		status = *in.Status
	}
	return schemas.TeamMember{ID: 1, TeamID: teamID, UserID: userID, UserEmail: "owner@example.com", DisplayName: "Console User", Role: role, Status: status}, nil
}

func (f *fakeCatalogService) CreateTeamInvite(_ context.Context, teamID int64, in schemas.TeamInviteCreate, _ *int64) (schemas.TeamInvite, error) {
	return schemas.TeamInvite{ID: 1, TeamID: teamID, Email: in.Email, Role: in.Role, InviteToken: "ti_123", Status: "pending"}, nil
}

func (f *fakeCatalogService) ListTeamInvites(_ context.Context, teamID int64) ([]schemas.TeamInvite, error) {
	return []schemas.TeamInvite{
		{ID: 1, TeamID: teamID, Email: "new@example.com", Role: "member", InviteToken: "ti_123", Status: "pending"},
	}, nil
}

func (f *fakeCatalogService) AcceptTeamInvite(_ context.Context, _ string, userID int64) (schemas.TeamMember, error) {
	return schemas.TeamMember{ID: 3, TeamID: 9, UserID: userID, UserEmail: "owner@example.com", DisplayName: "Console User", Role: "member", Status: "active"}, nil
}

func (f *fakeCatalogService) GetWalletSummary(_ context.Context, ownerType string, ownerID int64) (schemas.Wallet, error) {
	return schemas.Wallet{
		ID:        11,
		OwnerType: ownerType,
		OwnerID:   ownerID,
		Currency:  "CNY",
		Balance:   100.5,
		Status:    "active",
	}, nil
}

func (f *fakeCatalogService) ListRechargeOrders(_ context.Context, _ string, _ int64) ([]schemas.RechargeOrder, error) {
	return []schemas.RechargeOrder{
		{ID: 15, OrderNo: "RO-15", OwnerType: "user", OwnerID: 7, Amount: 88, Currency: "CNY", Status: "paid", PaymentProvider: "stripe"},
	}, nil
}

func (f *fakeCatalogService) CreateRechargeOrder(_ context.Context, ownerType string, ownerID int64, amount float64, currency string, provider string, _ *int64) (schemas.RechargeOrder, map[string]any, error) {
	return schemas.RechargeOrder{
			ID:              16,
			OrderNo:         "RO-16",
			OwnerType:       ownerType,
			OwnerID:         ownerID,
			Amount:          amount,
			Currency:        currency,
			Status:          "pending",
			PaymentProvider: provider,
		}, map[string]any{
			"payment_url": "/pay/" + provider + "/RO-16",
		}, nil
}

func (f *fakeCatalogService) GetRechargeOrder(_ context.Context, orderNo string) (schemas.RechargeOrder, error) {
	return schemas.RechargeOrder{
		ID:              16,
		OrderNo:         orderNo,
		OwnerType:       "user",
		OwnerID:         7,
		Amount:          66,
		Currency:        "CNY",
		Status:          "pending",
		PaymentProvider: "stripe",
	}, nil
}

func (f *fakeCatalogService) MarkRechargeOrderPaid(_ context.Context, provider string, eventID string, orderNo string, _ string, _ map[string]any) (schemas.RechargeOrder, bool, error) {
	return schemas.RechargeOrder{
		ID:              16,
		OrderNo:         orderNo,
		OwnerType:       "user",
		OwnerID:         7,
		Amount:          66,
		Currency:        "CNY",
		Status:          "paid",
		PaymentProvider: provider,
	}, eventID != "duplicate", nil
}

func (f *fakeCatalogService) CheckAPIKeyWallet(_ context.Context, _ schemas.APIKey) error {
	return f.walletCheckErr
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

	req = httptest.NewRequest(http.MethodGet, "/api/health/detail", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/api/health/detail status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
}

func TestProviderModelEndpoints(t *testing.T) {
	svc := &fakeCatalogService{
		providers: []schemas.Provider{
			{Name: "p1", Type: "openai", IsActive: true},
		},
		models: []schemas.Model{
			{ID: 1, ProviderName: "p1", Name: "gpt-4o", IsActive: true},
			{ID: 2, ProviderName: "p1", Name: "gpt-4o-mini", IsActive: true},
			{ID: 3, ProviderName: "p2", Name: "gemini-2.5-pro", IsActive: true},
		},
		supportedModels: map[string][]string{
			"p1": {"remote-a", "remote-b", "remote-c"},
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
	if !ok || len(models) != 3 {
		t.Fatalf("unexpected supported-models payload: %+v", payload)
	}
}

func TestProviderSupportedModelsEndpointReturnsNotImplemented(t *testing.T) {
	svc := &fakeCatalogService{
		supportedModelsErr: services.ErrNotImplemented,
	}
	r := NewRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/providers/p1/supported-models", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("supported-models status=%d want=501 body=%s", rr.Code, rr.Body.String())
	}
}

func TestProviderRemoteModelSyncEndpoints(t *testing.T) {
	svc := &fakeCatalogService{
		providers: []schemas.Provider{
			{Name: "p1", Type: "openai", IsActive: true},
		},
		remoteModels: map[string][]services.RemoteProviderModel{
			"p1": {
				{
					ProviderName:     "p1",
					ProviderType:     "openai",
					ModelName:        "remote-a",
					LocalName:        "remote-a",
					RemoteIdentifier: "remote-a",
				},
			},
		},
	}
	r := NewRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/providers/p1/remote-models?refresh=true", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("remote-models status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	var remotePayload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &remotePayload); err != nil {
		t.Fatalf("decode remote-models payload: %v", err)
	}
	if models, ok := remotePayload["models"].([]any); !ok || len(models) != 1 {
		t.Fatalf("unexpected remote-models payload: %+v", remotePayload)
	}

	req = httptest.NewRequest(http.MethodPost, "/providers/p1/models/sync", strings.NewReader(`{"default_new_model_active":true}`))
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("provider models sync status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	var run services.ModelUpdateRun
	if err := json.Unmarshal(rr.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode sync run: %v", err)
	}
	if run.ProviderName != "p1" || len(run.Added) != 1 || len(run.Disabled) != 1 {
		t.Fatalf("unexpected sync run: %+v", run)
	}

	req = httptest.NewRequest(http.MethodGet, "/model-updates/runs", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("model update runs status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
}

func TestMonitorChannelLoadEndpointWithoutSupport(t *testing.T) {
	svc := &fakeCatalogService{}
	r := NewRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/monitor/channel-load", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("channel-load status=%d want=501 body=%s", rr.Code, rr.Body.String())
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

func TestRequestLoggingIncludesRouteDecisionWithoutSecrets(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	svc := &fakeCatalogService{
		onChat: func(ctx context.Context, providerHint string, payload map[string]any) {
			logging.WithAttrs(ctx,
				slog.String("provider", "p1"),
				slog.String("model", "gpt-4o"),
				slog.String("routing_mode", "provider_hint"),
				slog.Bool("stream", false),
			)
		},
	}
	r := NewRouterWithOptions(svc, RouterOptions{Logger: logger})

	body := []byte(`{"model":"p1/gpt-4o","messages":[{"role":"user","content":"secret prompt"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer super-secret")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	logLine := buf.String()
	if !strings.Contains(logLine, `"msg":"http request"`) {
		t.Fatalf("log missing request message: %s", logLine)
	}
	for _, want := range []string{`"provider":"p1"`, `"model":"gpt-4o"`, `"routing_mode":"provider_hint"`, `"status":200`, `"method":"POST"`, `"path":"/v1/chat/completions"`} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log missing %s: %s", want, logLine)
		}
	}
	for _, banned := range []string{"super-secret", "secret prompt", "Authorization"} {
		if strings.Contains(logLine, banned) {
			t.Fatalf("log should not contain %q: %s", banned, logLine)
		}
	}
}

func TestRequestLoggingIncludesErrorSummary(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	svc := &fakeCatalogService{chatErr: services.ErrNotFound}
	r := NewRouterWithOptions(svc, RouterOptions{Logger: logger})

	body := []byte(`{"model":"p1/missing","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=404 body=%s", rr.Code, rr.Body.String())
	}

	logLine := buf.String()
	if !strings.Contains(logLine, `"status":404`) {
		t.Fatalf("log missing 404 status: %s", logLine)
	}
	if !strings.Contains(logLine, `"error":"model not found"`) {
		t.Fatalf("log missing error summary: %s", logLine)
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

func TestPluginTTSEndpoints(t *testing.T) {
	svc := &fakeCatalogService{
		ttsPlugins: []map[string]any{
			{
				"name":          "qwen_tts",
				"default_model": "qwen-tts-latest",
				"models":        []string{"qwen-tts-latest"},
			},
		},
		ttsVoices: map[string][]map[string]any{
			"qwen_tts::qwen-tts-latest": {
				{"id": "xiaoyun", "display_name": "Xiao Yun", "downloaded": true},
			},
		},
	}
	r := NewRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/plugins/tts", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/plugins/tts status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/plugins/tts/qwen_tts/voices?model_id=qwen-tts-latest", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/plugins/tts/{plugin}/voices status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "xiaoyun") {
		t.Fatalf("voices response missing expected voice: %s", rr.Body.String())
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

	req = httptest.NewRequest(http.MethodGet, "/auth/oauth/openrouter/accounts?provider_name=openrouter-main", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/auth/oauth/{provider}/accounts status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	var oauthResp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &oauthResp); err != nil {
		t.Fatalf("decode /auth/oauth/{provider}/accounts payload: %v", err)
	}
	accountsAny, ok := oauthResp["accounts"].([]any)
	if !ok {
		t.Fatalf("oauth accounts payload missing accounts array: %+v", oauthResp)
	}
	oauthAccounts := make([]map[string]any, 0, len(accountsAny))
	for _, item := range accountsAny {
		m, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("oauth account item is not object: %+v", item)
		}
		oauthAccounts = append(oauthAccounts, m)
	}
	if len(oauthAccounts) == 0 {
		t.Fatalf("expected oauth accounts payload")
	}
	forbiddenFields := []string{"access_token", "refresh_token", "api_key"}
	for _, field := range forbiddenFields {
		if _, exists := oauthAccounts[0][field]; exists {
			t.Fatalf("oauth accounts response leaked %s", field)
		}
	}

	req = httptest.NewRequest(http.MethodPatch, "/auth/oauth/openrouter/accounts/1?provider_name=openrouter-main", strings.NewReader(`{"account_name":"primary","is_active":true}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH /auth/oauth/{provider}/accounts/{id} status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/auth/oauth/openrouter/accounts/1/default?provider_name=openrouter-main", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /auth/oauth/{provider}/accounts/{id}/default status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/auth/oauth/openrouter/accounts/1?provider_name=openrouter-main", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("DELETE /auth/oauth/{provider}/accounts/{id} status=%d want=200 body=%s", rr.Code, rr.Body.String())
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

func TestAuthMiddlewareRejectsExpiredAndDisallowedIP(t *testing.T) {
	key := "auth-expired-key"
	expiredAt := time.Now().UTC().Add(-1 * time.Hour)
	svc := &fakeCatalogService{
		apiKeys: []schemas.APIKey{
			{
				ID:          1,
				Key:         &key,
				IsActive:    true,
				ExpiresAt:   &expiredAt,
				IPAllowlist: []string{"10.0.0.0/8"},
			},
		},
	}
	r := NewRouterWithOptions(svc, RouterOptions{
		RequireAuth:      true,
		AllowLocalNoAuth: false,
	})

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	req.RemoteAddr = "203.0.113.8:1234"
	req.Header.Set("Authorization", "Bearer "+key)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expired key status=%d want=403 body=%s", rr.Code, rr.Body.String())
	}

	validKey := "auth-valid-key"
	svc.apiKeys = []schemas.APIKey{
		{
			ID:          2,
			Key:         &validKey,
			IsActive:    true,
			IPAllowlist: []string{"10.0.0.0/8"},
		},
	}
	req = httptest.NewRequest(http.MethodGet, "/models", nil)
	req.RemoteAddr = "203.0.113.8:1234"
	req.Header.Set("X-Forwarded-For", "10.1.2.3")
	req.Header.Set("Authorization", "Bearer "+validKey)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("ip not allowed status=%d want=403 body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddlewareStreamingChatAccumulatesUsage(t *testing.T) {
	key := "stream-usage-key"
	quota := int64(100)
	svc := &fakeCatalogService{
		apiKeys: []schemas.APIKey{
			{ID: 99, Key: &key, IsActive: true, QuotaTokensMonth: &quota},
		},
	}
	r := NewRouterWithOptions(svc, RouterOptions{
		RequireAuth:      true,
		AllowLocalNoAuth: false,
	})

	body := []byte(`{"model":"p1/gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.8:1234"
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("stream /v1/chat/completions status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	if svc.usageCalls == 0 {
		t.Fatalf("expected streamed chat usage accumulation")
	}
	if svc.lastUsageAPIKeyID != 99 {
		t.Fatalf("unexpected usage api_key_id: %d", svc.lastUsageAPIKeyID)
	}
	if svc.lastUsageTokens != 42 {
		t.Fatalf("unexpected streamed usage tokens: %d", svc.lastUsageTokens)
	}
	if svc.lastUsageCost <= 0 {
		t.Fatalf("expected streamed usage cost > 0, got %f", svc.lastUsageCost)
	}
}

func TestConsoleRoutesUseCookieSession(t *testing.T) {
	svc := &fakeCatalogService{}
	r := NewRouter(svc)

	loginBody := []byte(`{"email":"owner@example.com","password":"secret123"}`)
	req := httptest.NewRequest(http.MethodPost, "/console/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/console/auth/login status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
	cookieHeader := rr.Header().Get("Set-Cookie")
	if !strings.Contains(cookieHeader, "console_session=") {
		t.Fatalf("expected console session cookie, got %q", cookieHeader)
	}

	req = httptest.NewRequest(http.MethodGet, "/console/auth/me", nil)
	req.Header.Set("Cookie", cookieHeader)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/console/auth/me status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/console/wallets/me", nil)
	req.Header.Set("Cookie", cookieHeader)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/console/wallets/me status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/console/orders", nil)
	req.Header.Set("Cookie", cookieHeader)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/console/orders status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/console/orders/recharge", bytes.NewReader([]byte(`{"amount":66,"currency":"CNY","payment_provider":"stripe"}`)))
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("/console/orders/recharge status=%d want=201 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/payments/callback/stripe", bytes.NewReader([]byte(`{"event_id":"evt-1","order_no":"RO-16","provider_trade_no":"pi_1"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/payments/callback/stripe status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/console/api-keys", nil)
	req.Header.Set("Cookie", cookieHeader)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/console/api-keys status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/console/wallets/teams/9", nil)
	req.Header.Set("Cookie", cookieHeader)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/console/wallets/teams/9 status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/console/users/7", bytes.NewReader([]byte(`{"status":"disabled"}`)))
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/console/users/7 status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/console/teams", bytes.NewReader([]byte(`{"name":"Growth","slug":"growth","description":"growth squad"}`)))
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("/console/teams status=%d want=201 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/console/teams/9/members", nil)
	req.Header.Set("Cookie", cookieHeader)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/console/teams/9/members status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/console/teams/9/members/7", bytes.NewReader([]byte(`{"role":"billing","status":"active"}`)))
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/console/teams/9/members/7 status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/console/teams/9/members", bytes.NewReader([]byte(`{"user_id":8,"role":"member"}`)))
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("/console/teams/9/members status=%d want=201 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/console/teams/9/orders/recharge", bytes.NewReader([]byte(`{"amount":128,"currency":"CNY","payment_provider":"stripe"}`)))
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("/console/teams/9/orders/recharge status=%d want=201 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/console/teams/9/invites", bytes.NewReader([]byte(`{"email":"new@example.com","role":"member"}`)))
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("/console/teams/9/invites status=%d want=201 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/console/teams/9/invites", nil)
	req.Header.Set("Cookie", cookieHeader)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/console/teams/9/invites status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/console/invites/accept", bytes.NewReader([]byte(`{"invite_token":"ti_123"}`)))
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/console/invites/accept status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddlewareRejectsWhenWalletIsInsufficient(t *testing.T) {
	key := "wallet-guard-key"
	svc := &fakeCatalogService{
		apiKeys: []schemas.APIKey{
			{ID: 55, Key: &key, IsActive: true},
		},
		walletCheckErr: errors.New("wallet balance insufficient"),
	}
	r := NewRouterWithOptions(svc, RouterOptions{
		RequireAuth:      true,
		AllowLocalNoAuth: false,
	})

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	req.RemoteAddr = "203.0.113.8:1234"
	req.Header.Set("Authorization", "Bearer "+key)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("wallet guard status=%d want=403 body=%s", rr.Code, rr.Body.String())
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

func TestNewPolicyAndQuotaEndpoints(t *testing.T) {
	svc := &fakeCatalogService{}
	r := NewRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/monitor/quota-details", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/quota-details status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/quota-details/export?format=json", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/quota-details/export status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monitor/budget-alerts", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/monitor/budget-alerts status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api-key-policy-templates", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/api-key-policy-templates status=%d want=200 body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/providers/openai/catalog-models/sync", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/providers/{name}/catalog-models/sync status=%d want=200 body=%s", rr.Code, rr.Body.String())
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
