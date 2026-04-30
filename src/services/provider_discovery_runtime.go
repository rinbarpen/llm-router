package services

import (
	"strings"
	"sync"
	"time"
)

type providerDiscoveryRuntime struct {
	mu    sync.Mutex
	ttl   time.Duration
	cache map[string]providerDiscoveryCacheEntry
}

type providerDiscoveryCacheEntry struct {
	models    []map[string]any
	expiresAt time.Time
}

func newProviderDiscoveryRuntime(ttl time.Duration) *providerDiscoveryRuntime {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &providerDiscoveryRuntime{
		ttl:   ttl,
		cache: map[string]providerDiscoveryCacheEntry{},
	}
}

func (r *providerDiscoveryRuntime) get(providerName string) ([]map[string]any, bool) {
	if r == nil || strings.TrimSpace(providerName) == "" {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.cache[providerName]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			delete(r.cache, providerName)
		}
		return nil, false
	}
	return cloneDiscoveredModels(entry.models), true
}

func (r *providerDiscoveryRuntime) put(providerName string, models []map[string]any) {
	if r == nil || strings.TrimSpace(providerName) == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache[providerName] = providerDiscoveryCacheEntry{
		models:    cloneDiscoveredModels(models),
		expiresAt: time.Now().Add(r.ttl),
	}
}

func (r *providerDiscoveryRuntime) invalidate(providerName string) {
	if r == nil || strings.TrimSpace(providerName) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cache, providerName)
}

func cloneDiscoveredModels(in []map[string]any) []map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(in))
	for _, row := range in {
		cloned := make(map[string]any, len(row))
		for k, v := range row {
			cloned[k] = v
		}
		out = append(out, cloned)
	}
	return out
}
