package providers

import "github.com/rinbarpen/llm-router/src/db"

// AzureOpenAIProviderClient currently follows OpenAI-compatible contract.
type AzureOpenAIProviderClient struct {
	*OpenAICompatibleProviderClient
}

func NewAzureOpenAIProviderClient(provider db.Provider) *AzureOpenAIProviderClient {
	return &AzureOpenAIProviderClient{OpenAICompatibleProviderClient: NewOpenAICompatibleProviderClient(provider)}
}
