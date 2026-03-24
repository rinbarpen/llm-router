package providers

import "github.com/rinbarpen/llm-router/backend/internal/db"

// VLLMProviderClient currently follows OpenAI-compatible contract.
type VLLMProviderClient struct {
	*OpenAICompatibleProviderClient
}

func NewVLLMProviderClient(provider db.Provider) *VLLMProviderClient {
	return &VLLMProviderClient{OpenAICompatibleProviderClient: NewOpenAICompatibleProviderClient(provider)}
}
