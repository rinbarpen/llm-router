package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ModelPricingInfo captures provider pricing snapshot.
type ModelPricingInfo struct {
	ModelName        string    `json:"model_name"`
	Provider         string    `json:"provider"`
	InputPricePer1K  float64   `json:"input_price_per_1k"`
	OutputPricePer1K float64   `json:"output_price_per_1k"`
	Source           string    `json:"source"`
	LastUpdated      time.Time `json:"last_updated"`
	Notes            string    `json:"notes,omitempty"`
}

// PricingService caches externally fetched pricing metadata.
type PricingService struct {
	client *http.Client
	ttl    time.Duration
	mu     sync.RWMutex
	cache  map[string]cachedPricing
}

type cachedPricing struct {
	item      ModelPricingInfo
	expiresAt time.Time
}

func NewPricingService(ttl time.Duration) *PricingService {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &PricingService{
		client: &http.Client{Timeout: 20 * time.Second},
		ttl:    ttl,
		cache:  map[string]cachedPricing{},
	}
}

func pricingKey(provider, model string) string {
	return strings.ToLower(strings.TrimSpace(provider)) + "/" + strings.ToLower(strings.TrimSpace(model))
}

func (s *PricingService) Get(provider, model string) (ModelPricingInfo, bool) {
	key := pricingKey(provider, model)
	now := time.Now()
	s.mu.RLock()
	entry, ok := s.cache[key]
	s.mu.RUnlock()
	if !ok || now.After(entry.expiresAt) {
		return ModelPricingInfo{}, false
	}
	return entry.item, true
}

func (s *PricingService) Set(item ModelPricingInfo) {
	key := pricingKey(item.Provider, item.ModelName)
	if item.LastUpdated.IsZero() {
		item.LastUpdated = time.Now()
	}
	s.mu.Lock()
	s.cache[key] = cachedPricing{item: item, expiresAt: time.Now().Add(s.ttl)}
	s.mu.Unlock()
}

func (s *PricingService) FetchAndCache(ctx context.Context, url string) ([]ModelPricingInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(url), nil)
	if err != nil {
		return nil, fmt.Errorf("build pricing request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pricing request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pricing endpoint status: %d", resp.StatusCode)
	}
	var items []ModelPricingInfo
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("decode pricing payload: %w", err)
	}
	for _, item := range items {
		s.Set(item)
	}
	return items, nil
}
