package providers

import "github.com/rinbarpen/llm-router/src/db"

// TransformersProviderClient currently follows OpenAI-compatible contract.
type TransformersProviderClient struct {
	*OpenAICompatibleProviderClient
}

func NewTransformersProviderClient(provider db.Provider) *TransformersProviderClient {
	return &TransformersProviderClient{OpenAICompatibleProviderClient: NewOpenAICompatibleProviderClient(provider)}
}
