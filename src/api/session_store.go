package api

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type SessionData struct {
	Token              string
	APIKeyID           int64
	ProviderName       string
	ModelName          string
	StrongProviderName string
	StrongModelName    string
	WeakProviderName   string
	WeakModelName      string
	CreatedAt          time.Time
}

type SessionStore interface {
	Create(apiKeyID int64) (SessionData, error)
	Get(token string) (SessionData, bool)
	Delete(token string)
	BindModel(token string, providerName string, modelName string) bool
	BindProfileModel(token string, providerName string, modelName string, bindingType string) bool
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

func (s *MemorySessionStore) BindModel(token string, providerName string, modelName string) bool {
	return s.BindProfileModel(token, providerName, modelName, "default")
}

func (s *MemorySessionStore) BindProfileModel(token string, providerName string, modelName string, bindingType string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[token]
	if !ok {
		return false
	}
	switch bindingType {
	case "strong":
		item.StrongProviderName = providerName
		item.StrongModelName = modelName
	case "weak":
		item.WeakProviderName = providerName
		item.WeakModelName = modelName
	default:
		item.ProviderName = providerName
		item.ModelName = modelName
	}
	s.items[token] = item
	return true
}

func randomToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
