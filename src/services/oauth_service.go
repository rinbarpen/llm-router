package services

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// OAuthStateStore keeps temporary OAuth states in memory.
type OAuthStateStore struct {
	mu         sync.RWMutex
	states     map[string]time.Time
	defaultTTL time.Duration
}

func NewOAuthStateStore(defaultTTL time.Duration) *OAuthStateStore {
	if defaultTTL <= 0 {
		defaultTTL = 10 * time.Minute
	}
	return &OAuthStateStore{states: map[string]time.Time{}, defaultTTL: defaultTTL}
}

func (s *OAuthStateStore) NewState() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	state := hex.EncodeToString(buf)
	s.mu.Lock()
	s.states[state] = time.Now().Add(s.defaultTTL)
	s.mu.Unlock()
	return state
}

func (s *OAuthStateStore) ValidateAndConsume(state string) bool {
	now := time.Now()
	s.mu.Lock()
	expiresAt, ok := s.states[state]
	if ok {
		delete(s.states, state)
	}
	s.mu.Unlock()
	return ok && now.Before(expiresAt)
}
