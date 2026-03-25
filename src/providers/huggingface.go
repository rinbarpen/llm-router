package providers

import "github.com/rinbarpen/llm-router/src/db"

// HuggingFaceProviderClient currently follows OpenAI-compatible contract.
type HuggingFaceProviderClient struct {
	*OpenAICompatibleProviderClient
}

func NewHuggingFaceProviderClient(provider db.Provider) *HuggingFaceProviderClient {
	return &HuggingFaceProviderClient{OpenAICompatibleProviderClient: NewOpenAICompatibleProviderClient(provider)}
}
