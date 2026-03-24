package providers

import "github.com/rinbarpen/llm-router/backend/internal/db"

type ClaudeCodeCLIProviderClient struct {
	*CodeCLIProviderClient
}

func NewClaudeCodeCLIProviderClient(provider db.Provider) *ClaudeCodeCLIProviderClient {
	return &ClaudeCodeCLIProviderClient{CodeCLIProviderClient: NewCodeCLIProviderClient(provider)}
}
