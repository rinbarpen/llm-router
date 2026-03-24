package providers

import "github.com/rinbarpen/llm-router/backend/internal/db"

// OllamaProviderClient currently follows OpenAI-compatible contract.
type OllamaProviderClient struct {
	*OpenAICompatibleProviderClient
}

func NewOllamaProviderClient(provider db.Provider) *OllamaProviderClient {
	return &OllamaProviderClient{OpenAICompatibleProviderClient: NewOpenAICompatibleProviderClient(provider)}
}
