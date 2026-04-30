package services_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rinbarpen/llm-router/src/config"
	"github.com/rinbarpen/llm-router/src/db"
	"github.com/rinbarpen/llm-router/src/migrate"
	"github.com/rinbarpen/llm-router/src/schemas"
	"github.com/rinbarpen/llm-router/src/services"
)

func TestOpenAIChatCompletionsUsesModelPriorityForUnqualifiedModel(t *testing.T) {
	ctx := context.Background()
	svc := newPriorityRoutingTestService(t, ctx)

	srv, calledKeys, calledKeysMu := newPriorityRoutingChatServer()
	defer srv.Close()

	createPriorityRoutingProviderAndModel(t, ctx, svc, "aaa-low", srv.URL, "low-key", "m", 1)
	createPriorityRoutingProviderAndModel(t, ctx, svc, "zzz-high", srv.URL, "high-key", "m", 100)

	_, err := svc.OpenAIChatCompletions(ctx, "", map[string]any{
		"model":    "m",
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	})
	if err != nil {
		t.Fatalf("OpenAIChatCompletions() error = %v", err)
	}

	assertPriorityRoutingCalledKey(t, calledKeys, calledKeysMu, "high-key")
}

func TestGeminiGenerateContentUsesModelPriorityForUnqualifiedModel(t *testing.T) {
	ctx := context.Background()
	svc := newPriorityRoutingTestService(t, ctx)

	srv, calledKeys, calledKeysMu := newPriorityRoutingChatServer()
	defer srv.Close()

	createPriorityRoutingProviderAndModel(t, ctx, svc, "aaa-low", srv.URL, "low-key", "m", 1)
	createPriorityRoutingProviderAndModel(t, ctx, svc, "zzz-high", srv.URL, "high-key", "m", 100)

	_, err := svc.GeminiGenerateContent(ctx, "m", map[string]any{
		"contents": []any{
			map[string]any{
				"role": "user",
				"parts": []any{
					map[string]any{"text": "hello"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("GeminiGenerateContent() error = %v", err)
	}

	assertPriorityRoutingCalledKey(t, calledKeys, calledKeysMu, "high-key")
}

func TestClaudeMessagesUsesModelPriorityForUnqualifiedModel(t *testing.T) {
	ctx := context.Background()
	svc := newPriorityRoutingTestService(t, ctx)

	srv, calledKeys, calledKeysMu := newPriorityRoutingChatServer()
	defer srv.Close()

	createPriorityRoutingProviderAndModel(t, ctx, svc, "aaa-low", srv.URL, "low-key", "m", 1)
	createPriorityRoutingProviderAndModel(t, ctx, svc, "zzz-high", srv.URL, "high-key", "m", 100)

	_, err := svc.ClaudeMessages(ctx, map[string]any{
		"model": "m",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	})
	if err != nil {
		t.Fatalf("ClaudeMessages() error = %v", err)
	}

	assertPriorityRoutingCalledKey(t, calledKeys, calledKeysMu, "high-key")
}

func TestOpenAIChatCompletionsKeepsExplicitProviderOverModelPriority(t *testing.T) {
	ctx := context.Background()
	svc := newPriorityRoutingTestService(t, ctx)

	srv, calledKeys, calledKeysMu := newPriorityRoutingChatServer()
	defer srv.Close()

	createPriorityRoutingProviderAndModel(t, ctx, svc, "aaa-low", srv.URL, "low-key", "m", 1)
	createPriorityRoutingProviderAndModel(t, ctx, svc, "zzz-high", srv.URL, "high-key", "m", 100)

	_, err := svc.OpenAIChatCompletions(ctx, "", map[string]any{
		"model":    "aaa-low/m",
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	})
	if err != nil {
		t.Fatalf("OpenAIChatCompletions() error = %v", err)
	}

	assertPriorityRoutingCalledKey(t, calledKeys, calledKeysMu, "low-key")
}

func TestOpenAIChatCompletionsFallsBackByModelPriorityAfterRetryableError(t *testing.T) {
	ctx := context.Background()
	svc := newPriorityRoutingTestService(t, ctx)

	var mu sync.Mutex
	calledKeys := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer ")
		mu.Lock()
		calledKeys = append(calledKeys, key)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if key == "high-key" {
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "rate limited"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "chatcmpl-test",
			"object": "chat.completion",
			"choices": []any{
				map[string]any{
					"message": map[string]any{"role": "assistant", "content": "ok"},
				},
			},
		})
	}))
	defer srv.Close()

	createPriorityRoutingProviderAndModel(t, ctx, svc, "aaa-low", srv.URL, "low-key", "m", 1)
	createPriorityRoutingProviderAndModel(t, ctx, svc, "zzz-high", srv.URL, "high-key", "m", 100)

	_, err := svc.OpenAIChatCompletions(ctx, "", map[string]any{
		"model":    "m",
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	})
	if err != nil {
		t.Fatalf("OpenAIChatCompletions() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calledKeys) != 2 {
		t.Fatalf("called keys length = %d, want 2 (%v)", len(calledKeys), calledKeys)
	}
	if calledKeys[0] != "high-key" || calledKeys[1] != "low-key" {
		t.Fatalf("retryable fallback should follow model priority order, got %v", calledKeys)
	}
}

func newPriorityRoutingChatServer() (*httptest.Server, *[]string, *sync.Mutex) {
	var mu sync.Mutex
	calledKeys := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer ")
		mu.Lock()
		calledKeys = append(calledKeys, key)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "chatcmpl-test",
			"object": "chat.completion",
			"choices": []any{
				map[string]any{
					"message": map[string]any{"role": "assistant", "content": "ok"},
				},
			},
		})
	}))
	return srv, &calledKeys, &mu
}

func assertPriorityRoutingCalledKey(t *testing.T, calledKeys *[]string, mu *sync.Mutex, want string) {
	t.Helper()
	mu.Lock()
	defer mu.Unlock()
	if len(*calledKeys) != 1 {
		t.Fatalf("called keys length = %d, want 1 (%v)", len(*calledKeys), *calledKeys)
	}
	if (*calledKeys)[0] != want {
		t.Fatalf("unqualified model should use highest model priority provider, got key %q", (*calledKeys)[0])
	}
}

func newPriorityRoutingTestService(t *testing.T, ctx context.Context) *services.CatalogService {
	t.Helper()
	store, err := db.Connect(ctx, filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(store.Close)
	if err := migrate.Bootstrap(ctx, store, config.Config{
		MigrateFromSQLite: false,
		ModelConfigPath:   filepath.Join(t.TempDir(), "missing-router.toml"),
	}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	return services.NewCatalogService(store)
}

func createPriorityRoutingProviderAndModel(t *testing.T, ctx context.Context, svc *services.CatalogService, providerName string, baseURL string, apiKey string, modelName string, priority int64) {
	t.Helper()
	if _, err := svc.CreateProvider(ctx, schemas.ProviderCreate{
		Name:    providerName,
		Type:    "openai",
		BaseURL: &baseURL,
		APIKey:  &apiKey,
	}); err != nil {
		t.Fatalf("CreateProvider(%s) error = %v", providerName, err)
	}
	if _, err := svc.CreateModel(ctx, schemas.ModelCreate{
		ProviderName: providerName,
		Name:         modelName,
		Config:       map[string]any{"priority": priority},
	}); err != nil {
		t.Fatalf("CreateModel(%s/%s) error = %v", providerName, modelName, err)
	}
}
