package services

import (
	"sync"
	"time"
)

// RateLimitConfig mirrors python service-level token bucket config.
type RateLimitConfig struct {
	MaxRequests int64
	PerSeconds  int64
	BurstSize   int64
}

type tokenBucket struct {
	maxRequests float64
	perSeconds  float64
	burstSize   float64
	refillRate  float64
	tokens      float64
	lastRefill  time.Time
	mu          sync.Mutex
}

func newTokenBucket(cfg RateLimitConfig) *tokenBucket {
	burst := cfg.BurstSize
	if burst <= 0 {
		burst = cfg.MaxRequests
	}
	if cfg.MaxRequests <= 0 {
		cfg.MaxRequests = 1
	}
	if cfg.PerSeconds <= 0 {
		cfg.PerSeconds = 1
	}
	return &tokenBucket{
		maxRequests: float64(cfg.MaxRequests),
		perSeconds:  float64(cfg.PerSeconds),
		burstSize:   float64(burst),
		refillRate:  float64(cfg.MaxRequests) / float64(cfg.PerSeconds),
		tokens:      float64(burst),
		lastRefill:  time.Now(),
	}
}

func (b *tokenBucket) refill(now time.Time) {
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}
	b.lastRefill = now
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.burstSize {
		b.tokens = b.burstSize
	}
}

func (b *tokenBucket) acquire(tokens int64) {
	if tokens <= 0 {
		return
	}
	need := float64(tokens)
	for {
		b.mu.Lock()
		b.refill(time.Now())
		if b.tokens >= need {
			b.tokens -= need
			b.mu.Unlock()
			return
		}
		deficit := need - b.tokens
		waitSeconds := deficit / b.refillRate
		b.mu.Unlock()
		if waitSeconds <= 0 {
			waitSeconds = 0.001
		}
		time.Sleep(time.Duration(waitSeconds * float64(time.Second)))
	}
}

// RateLimiterManager applies per-model token bucket controls.
type RateLimiterManager struct {
	mu      sync.RWMutex
	buckets map[int64]*tokenBucket
}

func NewRateLimiterManager() *RateLimiterManager {
	return &RateLimiterManager{buckets: map[int64]*tokenBucket{}}
}

func (m *RateLimiterManager) Upsert(modelID int64, cfg RateLimitConfig) {
	m.mu.Lock()
	m.buckets[modelID] = newTokenBucket(cfg)
	m.mu.Unlock()
}

func (m *RateLimiterManager) Remove(modelID int64) {
	m.mu.Lock()
	delete(m.buckets, modelID)
	m.mu.Unlock()
}

func (m *RateLimiterManager) Acquire(modelID int64, tokens int64) {
	m.mu.RLock()
	bucket := m.buckets[modelID]
	m.mu.RUnlock()
	if bucket == nil {
		return
	}
	bucket.acquire(tokens)
}
