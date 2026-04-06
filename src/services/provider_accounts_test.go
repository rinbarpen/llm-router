package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseProviderAccountsSortAndEnv(t *testing.T) {
	t.Setenv("TEST_PROVIDER_ACCOUNT_KEY", "env-key-1")
	target := chatTarget{
		ProviderName: "openrouter-main",
		ProviderSettings: map[string]any{
			"accounts": []any{
				map[string]any{"name": "low", "api_key": "k-low", "priority": float64(1)},
				map[string]any{"name": "high", "api_key_env": "TEST_PROVIDER_ACCOUNT_KEY", "priority": float64(10)},
				map[string]any{"name": "disabled", "api_key": "k-disabled", "is_active": false},
			},
		},
	}

	accounts := parseProviderAccounts(target)
	if len(accounts) != 2 {
		t.Fatalf("account count = %d, want 2", len(accounts))
	}
	if accounts[0].Name != "high" || accounts[0].APIKey != "env-key-1" {
		t.Fatalf("unexpected first account: %+v", accounts[0])
	}
	if accounts[1].Name != "low" {
		t.Fatalf("unexpected second account: %+v", accounts[1])
	}
}

func TestExecuteOpenAIRequestWithFailoverOnRetryable(t *testing.T) {
	var primaryCalls int32
	var backupCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		switch auth {
		case "Bearer primary-key":
			atomic.AddInt32(&primaryCalls, 1)
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
		case "Bearer backup-key":
			atomic.AddInt32(&backupCalls, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()

	svc := &CatalogService{accountRT: newProviderAccountRuntime()}
	target := chatTarget{
		ProviderName: "openrouter-main",
		ProviderSettings: map[string]any{
			"accounts": []any{
				map[string]any{"name": "primary", "api_key": "primary-key", "priority": float64(10), "cooldown_seconds": float64(1)},
				map[string]any{"name": "backup", "api_key": "backup-key", "priority": float64(1), "cooldown_seconds": float64(1)},
			},
		},
	}

	resp, err := svc.executeOpenAIRequestWithFailover(context.Background(), target, 2*time.Second, false, func(apiKey string) (*http.Request, error) {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/v1/chat/completions", strings.NewReader(`{}`))
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("unexpected body: %s", string(body))
	}
	if atomic.LoadInt32(&primaryCalls) != 1 || atomic.LoadInt32(&backupCalls) != 1 {
		t.Fatalf("unexpected calls primary=%d backup=%d", primaryCalls, backupCalls)
	}
}

func TestExecuteOpenAIRequestNoFailoverOnNonRetryable4xx(t *testing.T) {
	var primaryCalls int32
	var backupCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if auth == "Bearer primary-key" {
			atomic.AddInt32(&primaryCalls, 1)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad request"}`))
			return
		}
		if auth == "Bearer backup-key" {
			atomic.AddInt32(&backupCalls, 1)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := &CatalogService{accountRT: newProviderAccountRuntime()}
	target := chatTarget{
		ProviderName: "openrouter-main",
		ProviderSettings: map[string]any{
			"accounts": []any{
				map[string]any{"name": "primary", "api_key": "primary-key", "priority": float64(10)},
				map[string]any{"name": "backup", "api_key": "backup-key", "priority": float64(1)},
			},
		},
	}

	_, err := svc.executeOpenAIRequestWithFailover(context.Background(), target, 2*time.Second, false, func(apiKey string) (*http.Request, error) {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/v1/chat/completions", strings.NewReader(`{}`))
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return req, nil
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	var upErr *UpstreamStatusError
	if !strings.Contains(err.Error(), "bad request") && !strings.Contains(fmt.Sprintf("%T", err), "UpstreamStatusError") {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&primaryCalls) != 1 || atomic.LoadInt32(&backupCalls) != 0 {
		t.Fatalf("unexpected calls primary=%d backup=%d", primaryCalls, backupCalls)
	}
	_ = upErr
}

func TestExecuteOpenAIRequestReturns429WhenAllAccountsLimited(t *testing.T) {
	svc := &CatalogService{accountRT: newProviderAccountRuntime()}
	target := chatTarget{
		ProviderName: "openrouter-main",
		ProviderSettings: map[string]any{
			"accounts": []any{
				map[string]any{
					"name":             "only",
					"api_key":          "only-key",
					"priority":         float64(10),
					"max_requests":     float64(1),
					"per_seconds":      float64(3600),
					"cooldown_seconds": float64(1),
				},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	requestBuilder := func(apiKey string) (*http.Request, error) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/v1/chat/completions", strings.NewReader(`{}`))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return req, nil
	}
	firstResp, firstErr := svc.executeOpenAIRequestWithFailover(context.Background(), target, 2*time.Second, false, requestBuilder)
	if firstErr != nil {
		t.Fatalf("first request failed: %v", firstErr)
	}
	_ = firstResp.Body.Close()

	_, secondErr := svc.executeOpenAIRequestWithFailover(context.Background(), target, 2*time.Second, false, requestBuilder)
	if secondErr == nil {
		t.Fatalf("expected second request to be rate limited")
	}
	upErr, ok := secondErr.(*UpstreamStatusError)
	if !ok {
		t.Fatalf("unexpected error type: %T", secondErr)
	}
	if upErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status=%d want=429 detail=%s", upErr.StatusCode, upErr.Detail)
	}
}
