package services

import (
	"sync"
	"time"

	"github.com/rinbarpen/llm-router/backend/internal/db"
)

// CliSessionInfo maps router conversation to provider-native session id.
type CliSessionInfo struct {
	CLIID        string
	ProviderType db.ProviderType
	CreatedAt    time.Time
	TokenCount   int64
}

// CliConversationStore stores (provider, conversation_key) -> session.
type CliConversationStore struct {
	mu         sync.RWMutex
	store      map[string]CliSessionInfo
	defaultTTL time.Duration
}

func NewCliConversationStore(defaultTTL time.Duration) *CliConversationStore {
	if defaultTTL <= 0 {
		defaultTTL = 24 * time.Hour
	}
	return &CliConversationStore{store: map[string]CliSessionInfo{}, defaultTTL: defaultTTL}
}

func (s *CliConversationStore) key(providerType db.ProviderType, conversationKey string) string {
	return string(providerType) + ":" + conversationKey
}

func (s *CliConversationStore) Get(providerType db.ProviderType, conversationKey string) (CliSessionInfo, bool) {
	s.mu.RLock()
	info, ok := s.store[s.key(providerType, conversationKey)]
	s.mu.RUnlock()
	if !ok {
		return CliSessionInfo{}, false
	}
	if s.defaultTTL > 0 && time.Since(info.CreatedAt) > s.defaultTTL {
		s.Delete(providerType, conversationKey)
		return CliSessionInfo{}, false
	}
	return info, true
}

func (s *CliConversationStore) Set(providerType db.ProviderType, conversationKey string, cliID string, tokenCount int64) {
	s.mu.Lock()
	s.store[s.key(providerType, conversationKey)] = CliSessionInfo{
		CLIID:        cliID,
		ProviderType: providerType,
		CreatedAt:    time.Now(),
		TokenCount:   tokenCount,
	}
	s.mu.Unlock()
}

func (s *CliConversationStore) Delete(providerType db.ProviderType, conversationKey string) bool {
	key := s.key(providerType, conversationKey)
	s.mu.Lock()
	_, ok := s.store[key]
	if ok {
		delete(s.store, key)
	}
	s.mu.Unlock()
	return ok
}

func (s *CliConversationStore) UpdateTokenCount(providerType db.ProviderType, conversationKey string, tokenCount int64) bool {
	key := s.key(providerType, conversationKey)
	s.mu.Lock()
	info, ok := s.store[key]
	if ok {
		info.TokenCount = tokenCount
		s.store[key] = info
	}
	s.mu.Unlock()
	return ok
}

func (s *CliConversationStore) CleanupByConversationKey(conversationKey string) int {
	removed := 0
	s.mu.Lock()
	for k := range s.store {
		if len(k) >= len(conversationKey)+1 && k[len(k)-len(conversationKey):] == conversationKey {
			delete(s.store, k)
			removed++
		}
	}
	s.mu.Unlock()
	return removed
}
