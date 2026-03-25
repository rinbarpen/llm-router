package providers

import "github.com/rinbarpen/llm-router/src/db"

// CodexAppServerProviderClient reuses OpenAI-compatible invoke shape.
type CodexAppServerProviderClient struct {
	*OpenAICompatibleProviderClient
}

func NewCodexAppServerProviderClient(provider db.Provider) *CodexAppServerProviderClient {
	return &CodexAppServerProviderClient{OpenAICompatibleProviderClient: NewOpenAICompatibleProviderClient(provider)}
}
