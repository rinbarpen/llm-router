package providers

import "github.com/rinbarpen/llm-router/backend/internal/db"

type CodexCLIProviderClient struct {
	*CodeCLIProviderClient
}

func NewCodexCLIProviderClient(provider db.Provider) *CodexCLIProviderClient {
	return &CodexCLIProviderClient{CodeCLIProviderClient: NewCodeCLIProviderClient(provider)}
}
