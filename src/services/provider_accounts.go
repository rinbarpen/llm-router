package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type providerAccount struct {
	Name            string
	APIKey          string
	Source          string
	IsDefault       bool
	Priority        int64
	MaxRequests     int64
	PerSeconds      int64
	BurstSize       int64
	MaxInFlight     int64
	CooldownSeconds int64
}

type providerAccountRuntime struct {
	mu    sync.Mutex
	state map[string]*providerAccountState
}

type providerAccountState struct {
	mu            sync.Mutex
	bucket        *tokenBucket
	inflight      int64
	cooldownUntil map[string]time.Time
}

type providerAccountLease struct {
	release func()
}

func newProviderAccountRuntime() *providerAccountRuntime {
	return &providerAccountRuntime{state: map[string]*providerAccountState{}}
}

func (r *providerAccountRuntime) getState(providerName string, account providerAccount) *providerAccountState {
	key := providerName + "::" + account.Name
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.state[key]; s != nil {
		return s
	}
	var bucket *tokenBucket
	if account.MaxRequests > 0 && account.PerSeconds > 0 {
		bucket = newTokenBucket(RateLimitConfig{
			MaxRequests: account.MaxRequests,
			PerSeconds:  account.PerSeconds,
			BurstSize:   account.BurstSize,
		})
	}
	s := &providerAccountState{bucket: bucket, cooldownUntil: map[string]time.Time{}}
	r.state[key] = s
	return s
}

func (r *providerAccountRuntime) tryBegin(providerName string, account providerAccount) (*providerAccountLease, bool, string) {
	return r.tryBeginWithCooldownScope(providerName, account, providerName)
}

func (r *providerAccountRuntime) tryBeginWithCooldownScope(providerName string, account providerAccount, cooldownScope string) (*providerAccountLease, bool, string) {
	state := r.getState(providerName, account)
	now := time.Now()
	state.mu.Lock()
	defer state.mu.Unlock()
	cooldownScope = normalizeAccountCooldownScope(providerName, cooldownScope)
	if until := state.cooldownUntil[cooldownScope]; !until.IsZero() && now.Before(until) {
		return nil, false, "cooldown"
	}
	if state.bucket != nil && !state.bucket.tryAcquire(1) {
		return nil, false, "rate_limited"
	}
	if account.MaxInFlight > 0 && state.inflight >= account.MaxInFlight {
		return nil, false, "max_in_flight"
	}
	state.inflight++
	return &providerAccountLease{
		release: func() {
			state.mu.Lock()
			if state.inflight > 0 {
				state.inflight--
			}
			state.mu.Unlock()
		},
	}, true, ""
}

func (r *providerAccountRuntime) markFailure(providerName string, account providerAccount) {
	r.markFailureWithCooldownScope(providerName, account, providerName)
}

func (r *providerAccountRuntime) markFailureWithCooldownScope(providerName string, account providerAccount, cooldownScope string) {
	if account.CooldownSeconds <= 0 {
		return
	}
	state := r.getState(providerName, account)
	state.mu.Lock()
	cooldownScope = normalizeAccountCooldownScope(providerName, cooldownScope)
	state.cooldownUntil[cooldownScope] = time.Now().Add(time.Duration(account.CooldownSeconds) * time.Second)
	state.mu.Unlock()
}

func normalizeAccountCooldownScope(providerName string, cooldownScope string) string {
	if scope := strings.TrimSpace(cooldownScope); scope != "" {
		return scope
	}
	return strings.TrimSpace(providerName)
}

func parseProviderAccounts(target chatTarget) []providerAccount {
	settings := target.ProviderSettings
	raw, ok := settings["accounts"]
	if !ok {
		return nil
	}
	rows, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]providerAccount, 0, len(rows))
	for i, item := range rows {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if !asBoolDefault(m["is_active"], true) {
			continue
		}
		account := providerAccount{
			Name:            asStringDefault(m["name"], fmt.Sprintf("account-%d", i+1)),
			APIKey:          strings.TrimSpace(resolveAccountAPIKey(m)),
			Source:          asStringDefault(m["source"], "manual"),
			IsDefault:       asBoolDefault(m["is_default"], false),
			Priority:        asInt64Default(m["priority"], 0),
			MaxRequests:     asInt64Default(m["max_requests"], 0),
			PerSeconds:      asInt64Default(m["per_seconds"], 0),
			BurstSize:       asInt64Default(m["burst_size"], 0),
			MaxInFlight:     asInt64Default(m["max_in_flight"], 4),
			CooldownSeconds: asInt64Default(m["cooldown_seconds"], 30),
		}
		if account.APIKey == "" {
			continue
		}
		if account.BurstSize <= 0 {
			account.BurstSize = account.MaxRequests
		}
		out = append(out, account)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDefault != out[j].IsDefault {
			return out[i].IsDefault
		}
		if out[i].Priority == out[j].Priority {
			return out[i].Name < out[j].Name
		}
		return out[i].Priority > out[j].Priority
	})
	return out
}

func resolveAccountAPIKey(settings map[string]any) string {
	if raw, ok := settings["api_key"].(string); ok {
		if key := strings.TrimSpace(raw); key != "" {
			return key
		}
	}
	if raw, ok := settings["api_key_env"].(string); ok {
		if env := strings.TrimSpace(raw); env != "" {
			return strings.TrimSpace(os.Getenv(env))
		}
	}
	return ""
}

func asStringDefault(v any, fallback string) string {
	if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	return fallback
}

func asBoolDefault(v any, fallback bool) bool {
	switch t := v.(type) {
	case bool:
		return t
	default:
		return fallback
	}
}

func asInt64Default(v any, fallback int64) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case float64:
		return int64(t)
	case float32:
		return int64(t)
	default:
		return fallback
	}
}

func parseAPIKeyCSVAccounts(raw string) []providerAccount {
	parts := strings.Split(raw, ",")
	out := make([]providerAccount, 0, len(parts))
	for i, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		out = append(out, providerAccount{
			Name:            fmt.Sprintf("csv-%d", i+1),
			APIKey:          key,
			Source:          "csv",
			IsDefault:       i == 0,
			MaxInFlight:     4,
			CooldownSeconds: 30,
		})
	}
	return out
}

func dedupeAccounts(in []providerAccount) []providerAccount {
	seen := map[string]struct{}{}
	out := make([]providerAccount, 0, len(in))
	for _, item := range in {
		key := strings.TrimSpace(item.APIKey)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func sortAccounts(in []providerAccount) {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].IsDefault != in[j].IsDefault {
			return in[i].IsDefault
		}
		if in[i].Priority == in[j].Priority {
			return in[i].Name < in[j].Name
		}
		return in[i].Priority > in[j].Priority
	})
}

func (s *CatalogService) resolveRequestAccounts(ctx context.Context, target chatTarget) []providerAccount {
	out := make([]providerAccount, 0, 8)
	out = append(out, parseProviderAccounts(target)...)
	if target.ProviderAPIKey != nil {
		out = append(out, parseAPIKeyCSVAccounts(*target.ProviderAPIKey)...)
	}
	if s != nil && s.pool != nil {
		oauthAccounts, err := s.listProviderOAuthAccounts(ctx, target.ProviderName)
		if err == nil && len(oauthAccounts) > 0 {
			out = append(out, oauthAccounts...)
		}
	}
	out = dedupeAccounts(out)
	sortAccounts(out)
	return out
}

func isRetryableStatusCode(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

func summarizeUpstreamError(respBody []byte, statusCode int) string {
	out := map[string]any{}
	if len(respBody) > 0 && json.Valid(respBody) {
		_ = json.Unmarshal(respBody, &out)
	}
	detail := strings.TrimSpace(string(respBody))
	if len(out) > 0 {
		detail = fmt.Sprintf("%v", out)
	}
	if detail == "" {
		detail = http.StatusText(statusCode)
	}
	return detail
}

func (s *CatalogService) orderedProviderBaseURLs(providerName string, baseURL *string, settings map[string]any) []string {
	if s == nil || s.endpointRT == nil {
		cfg := parseEndpointPoolConfig(baseURL, settings)
		return cfg.urls
	}
	return s.endpointRT.order(providerName, baseURL, settings)
}

func (s *CatalogService) executeOpenAIRequestAcrossEndpoints(
	ctx context.Context,
	target chatTarget,
	timeout time.Duration,
	stream bool,
	buildReq func(providerBaseURL string, apiKey string) (*http.Request, error),
) (*http.Response, error) {
	baseURLs := s.orderedProviderBaseURLs(target.ProviderName, target.ProviderBaseURL, target.ProviderSettings)
	var lastErr error
	for _, providerBaseURL := range baseURLs {
		start := time.Now()
		cooldownScope := providerAccountCooldownScope(target.ProviderName, providerBaseURL, len(baseURLs))
		resp, err := s.executeOpenAIRequestWithFailoverAndCooldownScope(ctx, target, timeout, stream, cooldownScope, func(apiKey string) (*http.Request, error) {
			return buildReq(providerBaseURL, apiKey)
		})
		retryable := isRetryableOpenAIError(err)
		if s != nil && s.endpointRT != nil {
			s.endpointRT.finish(target.ProviderName, providerBaseURL, err == nil, time.Since(start), retryable)
		}
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, &UpstreamStatusError{StatusCode: http.StatusBadGateway, Detail: "all provider endpoints failed"}
}

func providerAccountCooldownScope(providerName string, providerBaseURL string, endpointCount int) string {
	if endpointCount > 1 && strings.EqualFold(strings.TrimSpace(providerName), "vapi") {
		return strings.TrimSpace(providerBaseURL)
	}
	return strings.TrimSpace(providerName)
}

func (s *CatalogService) executeOpenAIRequestWithFailover(
	ctx context.Context,
	target chatTarget,
	timeout time.Duration,
	stream bool,
	buildReq func(apiKey string) (*http.Request, error),
) (*http.Response, error) {
	return s.executeOpenAIRequestWithFailoverAndCooldownScope(ctx, target, timeout, stream, target.ProviderName, buildReq)
}

func (s *CatalogService) executeOpenAIRequestWithFailoverAndCooldownScope(
	ctx context.Context,
	target chatTarget,
	timeout time.Duration,
	stream bool,
	cooldownScope string,
	buildReq func(apiKey string) (*http.Request, error),
) (*http.Response, error) {
	accounts := s.resolveRequestAccounts(ctx, target)
	if len(accounts) == 0 {
		return nil, &UpstreamStatusError{StatusCode: http.StatusServiceUnavailable, Detail: "no available provider account"}
	}

	client := &http.Client{Timeout: timeout}
	if stream {
		client.Timeout = 0
	}

	skippedCount := 0
	var lastRetryable error
	for _, account := range accounts {
		lease, ok, _ := s.accountRT.tryBeginWithCooldownScope(target.ProviderName, account, cooldownScope)
		if !ok {
			skippedCount++
			continue
		}
		req, err := buildReq(account.APIKey)
		if err != nil {
			lease.release()
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			lease.release()
			s.accountRT.markFailureWithCooldownScope(target.ProviderName, account, cooldownScope)
			lastRetryable = fmt.Errorf("invoke upstream: %w", err)
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body = &releaseOnCloseBody{ReadCloser: resp.Body, release: lease.release}
			return resp, nil
		}
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		lease.release()
		detail := summarizeUpstreamError(respBody, resp.StatusCode)
		if isRetryableStatusCode(resp.StatusCode) {
			s.accountRT.markFailureWithCooldownScope(target.ProviderName, account, cooldownScope)
			lastRetryable = &UpstreamStatusError{StatusCode: resp.StatusCode, Detail: detail}
			continue
		}
		return nil, &UpstreamStatusError{StatusCode: resp.StatusCode, Detail: detail}
	}

	if skippedCount == len(accounts) {
		return nil, &UpstreamStatusError{StatusCode: http.StatusTooManyRequests, Detail: "all provider accounts are limited or cooling down"}
	}
	if lastRetryable != nil {
		return nil, lastRetryable
	}
	return nil, &UpstreamStatusError{StatusCode: http.StatusBadGateway, Detail: "all provider accounts failed"}
}

type releaseOnCloseBody struct {
	io.ReadCloser
	releaseOnce sync.Once
	release     func()
}

func (r *releaseOnCloseBody) Close() error {
	err := r.ReadCloser.Close()
	r.releaseOnce.Do(func() {
		if r.release != nil {
			r.release()
		}
	})
	return err
}
