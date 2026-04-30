package services

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultEndpointLatencyThreshold = 3 * time.Second
const defaultEndpointCooldown = 30 * time.Second

type providerEndpointRuntime struct {
	mu    sync.Mutex
	state map[string]map[string]*providerEndpointState
}

type providerEndpointState struct {
	avgLatencyMS       float64
	samples            int64
	lastRetryableError time.Time
	lastUpdatedAt      time.Time
}

type endpointPoolConfig struct {
	urls             []string
	latencyThreshold time.Duration
	cooldown         time.Duration
}

func newProviderEndpointRuntime() *providerEndpointRuntime {
	return &providerEndpointRuntime{
		state: map[string]map[string]*providerEndpointState{},
	}
}

func (r *providerEndpointRuntime) order(providerName string, baseURL *string, settings map[string]any) []string {
	cfg := parseEndpointPoolConfig(baseURL, settings)
	if len(cfg.urls) <= 1 {
		return cfg.urls
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	type rankedEndpoint struct {
		url         string
		idx         int
		coolingDown bool
		degraded    bool
		latencyMS   float64
	}
	ranked := make([]rankedEndpoint, 0, len(cfg.urls))
	for idx, item := range cfg.urls {
		st := r.getStateLocked(providerName, item)
		coolingDown := !st.lastRetryableError.IsZero() && now.Before(st.lastRetryableError.Add(cfg.cooldown))
		degraded := st.avgLatencyMS > 0 && st.avgLatencyMS > float64(cfg.latencyThreshold/time.Millisecond)
		ranked = append(ranked, rankedEndpoint{
			url:         item,
			idx:         idx,
			coolingDown: coolingDown,
			degraded:    degraded,
			latencyMS:   st.avgLatencyMS,
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].coolingDown != ranked[j].coolingDown {
			return !ranked[i].coolingDown
		}
		if ranked[i].degraded != ranked[j].degraded {
			return !ranked[i].degraded
		}
		latI, latJ := ranked[i].latencyMS, ranked[j].latencyMS
		if latI == 0 && latJ != 0 {
			return false
		}
		if latI != 0 && latJ == 0 {
			return true
		}
		if latI != latJ {
			return latI < latJ
		}
		return ranked[i].idx < ranked[j].idx
	})

	out := make([]string, 0, len(ranked))
	for _, item := range ranked {
		out = append(out, item.url)
	}
	return out
}

func (r *providerEndpointRuntime) finish(providerName string, endpoint string, success bool, duration time.Duration, retryable bool) {
	if strings.TrimSpace(providerName) == "" || strings.TrimSpace(endpoint) == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	st := r.getStateLocked(providerName, endpoint)
	latencyMS := float64(duration / time.Millisecond)
	if duration > 0 && latencyMS <= 0 {
		latencyMS = 1
	}
	if latencyMS > 0 {
		if st.samples == 0 || st.avgLatencyMS == 0 {
			st.avgLatencyMS = latencyMS
		} else {
			st.avgLatencyMS = st.avgLatencyMS*0.7 + latencyMS*0.3
		}
		st.samples++
	}
	if success {
		st.lastRetryableError = time.Time{}
	} else if retryable {
		st.lastRetryableError = time.Now()
	}
	st.lastUpdatedAt = time.Now()
}

func (r *providerEndpointRuntime) getStateLocked(providerName string, endpoint string) *providerEndpointState {
	providerState := r.state[providerName]
	if providerState == nil {
		providerState = map[string]*providerEndpointState{}
		r.state[providerName] = providerState
	}
	st := providerState[endpoint]
	if st == nil {
		st = &providerEndpointState{}
		providerState[endpoint] = st
	}
	return st
}

func parseEndpointPoolConfig(baseURL *string, settings map[string]any) endpointPoolConfig {
	cfg := endpointPoolConfig{
		latencyThreshold: defaultEndpointLatencyThreshold,
		cooldown:         defaultEndpointCooldown,
	}
	if settings != nil {
		if v := parseDurationSettingMS(settings["latency_degrade_threshold_ms"]); v > 0 {
			cfg.latencyThreshold = v
		}
		if v := parseDurationSettingSeconds(settings["cooldown_seconds"]); v > 0 {
			cfg.cooldown = v
		}
	}

	out := make([]string, 0, 4)
	seen := map[string]struct{}{}
	appendURL := func(raw string) {
		item := strings.TrimSpace(raw)
		if item == "" {
			return
		}
		item = strings.TrimRight(item, "/")
		if _, ok := seen[item]; ok {
			return
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}

	if settings != nil {
		switch rows := settings["api_base_urls"].(type) {
		case []any:
			for _, item := range rows {
				if s, ok := item.(string); ok {
					appendURL(s)
				}
			}
		case []string:
			for _, item := range rows {
				appendURL(item)
			}
		case string:
			for _, item := range splitSettingCSV(rows) {
				appendURL(item)
			}
		}
	}
	if baseURL != nil {
		appendURL(*baseURL)
	}
	if len(out) == 0 {
		appendURL("https://api.openai.com/v1")
	}
	cfg.urls = out
	return cfg
}

func parseDurationSettingMS(v any) time.Duration {
	switch item := v.(type) {
	case int:
		return time.Duration(item) * time.Millisecond
	case int64:
		return time.Duration(item) * time.Millisecond
	case float64:
		return time.Duration(item) * time.Millisecond
	default:
		return 0
	}
}

func parseDurationSettingSeconds(v any) time.Duration {
	switch item := v.(type) {
	case int:
		return time.Duration(item) * time.Second
	case int64:
		return time.Duration(item) * time.Second
	case float64:
		return time.Duration(item) * time.Second
	default:
		return 0
	}
}

func splitSettingCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, item := range parts {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func isRetryableOpenAIError(err error) bool {
	if err == nil {
		return false
	}
	var upstreamErr *UpstreamStatusError
	if errors.As(err, &upstreamErr) {
		return isRetryableStatusCode(upstreamErr.StatusCode)
	}
	return true
}

func shouldRetryCatalogEndpoint(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}
