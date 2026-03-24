package api

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type SessionData struct {
	Token     string
	APIKeyID  int64
	CreatedAt time.Time
}

type SessionStore interface {
	Create(apiKeyID int64) (SessionData, error)
	Get(token string) (SessionData, bool)
	Delete(token string)
}

type MemorySessionStore struct {
	mu    sync.RWMutex
	items map[string]SessionData
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		items: map[string]SessionData{},
	}
}

func (s *MemorySessionStore) Create(apiKeyID int64) (SessionData, error) {
	token, err := randomToken(24)
	if err != nil {
		return SessionData{}, err
	}
	data := SessionData{
		Token:     token,
		APIKeyID:  apiKeyID,
		CreatedAt: time.Now().UTC(),
	}
	s.mu.Lock()
	s.items[token] = data
	s.mu.Unlock()
	return data, nil
}

func (s *MemorySessionStore) Get(token string) (SessionData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.items[token]
	return data, ok
}

func (s *MemorySessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.items, token)
	s.mu.Unlock()
}

func randomToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
