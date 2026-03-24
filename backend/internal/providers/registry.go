package providers

import (
	"fmt"
	"sync"

	"github.com/rinbarpen/llm-router/backend/internal/db"
)

type clientFactory func(provider db.Provider) BaseProviderClient

var clientMapping = map[db.ProviderType]clientFactory{
	db.ProviderTypeRemoteHTTP:    func(p db.Provider) BaseProviderClient { return NewRemoteHTTPProviderClient(p) },
	db.ProviderTypeCustomHTTP:    func(p db.Provider) BaseProviderClient { return NewRemoteHTTPProviderClient(p) },
	db.ProviderTypeTransformers:  newOpenAICompatibleAlias,
	db.ProviderTypeOllama:        newOpenAICompatibleAlias,
	db.ProviderTypeVLLM:          newOpenAICompatibleAlias,
	db.ProviderTypeOpenAI:        newOpenAICompatibleAlias,
	db.ProviderTypeGrok:          newOpenAICompatibleAlias,
	db.ProviderTypeDeepseek:      newOpenAICompatibleAlias,
	db.ProviderTypeQwen:          newOpenAICompatibleAlias,
	db.ProviderTypeKimi:          newOpenAICompatibleAlias,
	db.ProviderTypeGLM:           newOpenAICompatibleAlias,
	db.ProviderTypeOpenRouter:    newOpenAICompatibleAlias,
	db.ProviderTypeAzureOpenAI:   newOpenAICompatibleAlias,
	db.ProviderTypeHuggingFace:   newOpenAICompatibleAlias,
	db.ProviderTypeMiniMax:       newOpenAICompatibleAlias,
	db.ProviderTypeDoubao:        newOpenAICompatibleAlias,
	db.ProviderTypeGroq:          newOpenAICompatibleAlias,
	db.ProviderTypeSiliconFlow:   newOpenAICompatibleAlias,
	db.ProviderTypeAIHubMix:      newOpenAICompatibleAlias,
	db.ProviderTypeVolcengine:    newOpenAICompatibleAlias,
	db.ProviderTypeGemini:        func(p db.Provider) BaseProviderClient { return NewGeminiProviderClient(p) },
	db.ProviderTypeClaude:        func(p db.Provider) BaseProviderClient { return NewAnthropicProviderClient(p) },
	db.ProviderTypeCodexCLI:      func(p db.Provider) BaseProviderClient { return NewCodexCLIProviderClient(p) },
	db.ProviderTypeClaudeCodeCLI: func(p db.Provider) BaseProviderClient { return NewClaudeCodeCLIProviderClient(p) },
	db.ProviderTypeOpenCodeCLI:   func(p db.Provider) BaseProviderClient { return NewOpenCodeCLIProviderClient(p) },
	db.ProviderTypeKimiCodeCLI:   func(p db.Provider) BaseProviderClient { return NewKimiCodeCLIProviderClient(p) },
	db.ProviderTypeQwenCodeCLI:   func(p db.Provider) BaseProviderClient { return NewQwenCodeCLIProviderClient(p) },
}

// ProviderRegistry caches provider clients keyed by provider ID.
type ProviderRegistry struct {
	mu      sync.RWMutex
	clients map[int64]BaseProviderClient
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{clients: map[int64]BaseProviderClient{}}
}

func (r *ProviderRegistry) Get(provider db.Provider) (BaseProviderClient, error) {
	r.mu.RLock()
	if c, ok := r.clients[provider.ID]; ok {
		r.mu.RUnlock()
		return c, nil
	}
	r.mu.RUnlock()

	factory, ok := clientMapping[provider.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
	client := factory(provider)

	r.mu.Lock()
	r.clients[provider.ID] = client
	r.mu.Unlock()
	return client, nil
}

func (r *ProviderRegistry) Remove(providerID int64) {
	r.mu.Lock()
	delete(r.clients, providerID)
	r.mu.Unlock()
}

func (r *ProviderRegistry) Clear() {
	r.mu.Lock()
	r.clients = map[int64]BaseProviderClient{}
	r.mu.Unlock()
}
