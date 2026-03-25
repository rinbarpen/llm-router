package services

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value     any
	expiresAt time.Time
	createdAt time.Time
}

func (e cacheEntry) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && now.After(e.expiresAt)
}

// CacheService is a small in-memory TTL cache.
type CacheService struct {
	mu         sync.RWMutex
	cache      map[string]cacheEntry
	defaultTTL time.Duration
	hits       int64
	misses     int64
}

func NewCacheService(defaultTTL time.Duration) *CacheService {
	if defaultTTL <= 0 {
		defaultTTL = 30 * time.Second
	}
	return &CacheService{cache: map[string]cacheEntry{}, defaultTTL: defaultTTL}
}

func (c *CacheService) Set(key string, value any, ttl time.Duration) {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	entry := cacheEntry{value: value, createdAt: time.Now(), expiresAt: time.Now().Add(ttl)}
	c.mu.Lock()
	c.cache[key] = entry
	c.mu.Unlock()
}

func (c *CacheService) Get(key string) (any, bool) {
	now := time.Now()
	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()
	if !ok {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}
	if entry.expired(now) {
		c.mu.Lock()
		delete(c.cache, key)
		c.misses++
		c.mu.Unlock()
		return nil, false
	}
	c.mu.Lock()
	c.hits++
	c.mu.Unlock()
	return entry.value, true
}

func (c *CacheService) Delete(key string) {
	c.mu.Lock()
	delete(c.cache, key)
	c.mu.Unlock()
}

func (c *CacheService) InvalidateAll() {
	c.mu.Lock()
	c.cache = map[string]cacheEntry{}
	c.mu.Unlock()
}

func (c *CacheService) CleanupExpired() int {
	now := time.Now()
	removed := 0
	c.mu.Lock()
	for k, entry := range c.cache {
		if entry.expired(now) {
			delete(c.cache, k)
			removed++
		}
	}
	c.mu.Unlock()
	return removed
}

type CacheStats struct {
	Hits         int64   `json:"hits"`
	Misses       int64   `json:"misses"`
	HitRate      float64 `json:"hit_rate"`
	TotalEntries int     `json:"total_entries"`
}

func (c *CacheService) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	total := c.hits + c.misses
	rate := 0.0
	if total > 0 {
		rate = float64(c.hits) * 100.0 / float64(total)
	}
	return CacheStats{
		Hits:         c.hits,
		Misses:       c.misses,
		HitRate:      rate,
		TotalEntries: len(c.cache),
	}
}
