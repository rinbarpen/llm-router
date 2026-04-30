package services

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rinbarpen/llm-router/src/schemas"
)

func TestFetchOpenAICompatibleModelsForBaseURLUsesAPIKeyEnv(t *testing.T) {
	const envName = "TEST_VAPI_DISCOVERY_KEY"
	const envValue = "secret-from-env"
	t.Setenv(envName, envValue)

	prevClient := providerCatalogHTTPClient
	providerCatalogHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+envValue {
			t.Fatalf("Authorization = %q, want Bearer %s", got, envValue)
		}
		return jsonResponse(`{"data":[{"id":"gpt-4o-mini"}]}`), nil
	})}
	defer func() { providerCatalogHTTPClient = prevClient }()

	models, err := fetchOpenAICompatibleModelsForBaseURL(context.Background(), schemas.Provider{
		Name: "vapi",
		Type: "openai",
		Settings: map[string]any{
			"api_key_env": envName,
		},
	}, "https://example.test")
	if err != nil {
		t.Fatalf("fetchOpenAICompatibleModelsForBaseURL() error = %v", err)
	}
	if len(models) != 1 || models[0]["model_name"] != "gpt-4o-mini" {
		t.Fatalf("unexpected models: %+v", models)
	}
}

func TestFetchProviderModelsLiveCachesResults(t *testing.T) {
	var hits atomic.Int64
	prevClient := providerCatalogHTTPClient
	providerCatalogHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		hits.Add(1)
		return jsonResponse(`{"data":[{"id":"cached-model"}]}`), nil
	})}
	defer func() { providerCatalogHTTPClient = prevClient }()

	svc := &CatalogService{
		endpointRT:  newProviderEndpointRuntime(),
		discoveryRT: newProviderDiscoveryRuntime(30 * time.Second),
	}
	baseURL := "https://example.test"
	provider := schemas.Provider{
		Name:     "vapi",
		Type:     "openai",
		BaseURL:  &baseURL,
		Settings: map[string]any{"api_key": "inline-key"},
	}

	first, err := svc.fetchProviderModelsLive(context.Background(), provider, false)
	if err != nil {
		t.Fatalf("first fetchProviderModelsLive() error = %v", err)
	}
	second, err := svc.fetchProviderModelsLive(context.Background(), provider, false)
	if err != nil {
		t.Fatalf("second fetchProviderModelsLive() error = %v", err)
	}

	if hits.Load() != 1 {
		t.Fatalf("server hits = %d, want 1", hits.Load())
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("unexpected models first=%+v second=%+v", first, second)
	}
}

func TestFetchOpenAICompatibleModelsForBaseURLHandlesBaseURLWithV1(t *testing.T) {
	prevClient := providerCatalogHTTPClient
	providerCatalogHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://api.openai.com/v1/models" {
			t.Fatalf("URL = %q, want %q", got, "https://api.openai.com/v1/models")
		}
		return jsonResponse(`{"data":[{"id":"gpt-4o"}]}`), nil
	})}
	defer func() { providerCatalogHTTPClient = prevClient }()

	models, err := fetchOpenAICompatibleModelsForBaseURL(context.Background(), schemas.Provider{
		Name: "openai",
		Type: "openai",
	}, "https://api.openai.com/v1")
	if err != nil {
		t.Fatalf("fetchOpenAICompatibleModelsForBaseURL() error = %v", err)
	}
	if len(models) != 1 || models[0]["model_name"] != "gpt-4o" {
		t.Fatalf("unexpected models: %+v", models)
	}
}

func TestOpenAICompatibleModelsEndpointSupportsProviderOverrides(t *testing.T) {
	endpoint := openAICompatibleModelsEndpoint(schemas.Provider{
		Name: "qwen",
		Type: "qwen",
	}, "https://dashscope.aliyuncs.com")
	if endpoint != "https://dashscope.aliyuncs.com/compatible-mode/v1/models" {
		t.Fatalf("qwen models endpoint = %q", endpoint)
	}

	endpoint = openAICompatibleModelsEndpoint(schemas.Provider{
		Name: "custom",
		Type: "openai",
		Settings: map[string]any{
			"models_endpoint": "/custom/models",
		},
	}, "https://example.test/api")
	if endpoint != "https://example.test/api/custom/models" {
		t.Fatalf("custom models endpoint = %q", endpoint)
	}
}

func TestFetchOpenAICompatibleModelsFallsBackToNextEndpoint(t *testing.T) {
	var firstHits atomic.Int64
	var secondHits atomic.Int64

	prevClient := providerCatalogHTTPClient
	providerCatalogHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "first.example.test":
			firstHits.Add(1)
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"bad gateway"}`)),
			}, nil
		case "second.example.test":
			secondHits.Add(1)
			return jsonResponse(`{"data":[{"id":"fallback-model"}]}`), nil
		default:
			t.Fatalf("unexpected host: %s", r.URL.Host)
			return nil, nil
		}
	})}
	defer func() { providerCatalogHTTPClient = prevClient }()

	svc := &CatalogService{
		endpointRT: newProviderEndpointRuntime(),
	}
	baseURL := "https://first.example.test"
	models, err := svc.fetchOpenAICompatibleModels(context.Background(), schemas.Provider{
		Name:    "vapi",
		Type:    "openai",
		BaseURL: &baseURL,
		Settings: map[string]any{
			"api_base_urls": []any{
				"https://first.example.test",
				"https://second.example.test",
			},
			"api_key": "inline-key",
		},
	})
	if err != nil {
		t.Fatalf("fetchOpenAICompatibleModels() error = %v", err)
	}
	if firstHits.Load() != 1 || secondHits.Load() != 1 {
		t.Fatalf("endpoint hits first=%d second=%d", firstHits.Load(), secondHits.Load())
	}
	if len(models) != 1 || models[0]["model_name"] != "fallback-model" {
		t.Fatalf("unexpected models: %+v", models)
	}
}

func TestDiscoveryOpenAICompatibleBaseURLsUsesProviderDefaults(t *testing.T) {
	urls := discoveryOpenAICompatibleBaseURLs(schemas.Provider{
		Name: "openrouter",
		Type: "openrouter",
	})
	if len(urls) != 1 || urls[0] != "https://openrouter.ai/api" {
		t.Fatalf("openrouter discovery urls = %#v", urls)
	}
}

func TestFetchProviderModelsReturnsNotImplementedForUnsupportedProvider(t *testing.T) {
	svc := &CatalogService{}
	_, err := svc.fetchProviderModels(context.Background(), schemas.Provider{
		Name: "azure-openai",
		Type: "azure_openai",
	})
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("fetchProviderModels() error = %v, want ErrNotImplemented", err)
	}
}

func TestResolveProviderAPIKey(t *testing.T) {
	const envName = "TEST_PROVIDER_API_KEY_ENV"
	const envValue = "env-key"
	t.Setenv(envName, envValue)

	inline := "provider-key"
	if got := resolveProviderAPIKey(schemas.Provider{APIKey: &inline}); got != inline {
		t.Fatalf("provider api key = %q, want %q", got, inline)
	}
	if got := resolveProviderAPIKey(schemas.Provider{Settings: map[string]any{"api_key": "settings-key"}}); got != "settings-key" {
		t.Fatalf("settings api_key = %q", got)
	}
	if got := resolveProviderAPIKey(schemas.Provider{Settings: map[string]any{"api_key_env": envName}}); got != envValue {
		t.Fatalf("settings api_key_env = %q, want %q", got, envValue)
	}
	if got := resolveProviderAPIKey(schemas.Provider{}); got != "" {
		t.Fatalf("empty provider api key = %q, want empty", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
