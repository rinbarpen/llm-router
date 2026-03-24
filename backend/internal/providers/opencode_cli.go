package providers

import "github.com/rinbarpen/llm-router/backend/internal/db"

type OpenCodeCLIProviderClient struct {
	*CodeCLIProviderClient
}

func NewOpenCodeCLIProviderClient(provider db.Provider) *OpenCodeCLIProviderClient {
	return &OpenCodeCLIProviderClient{CodeCLIProviderClient: NewCodeCLIProviderClient(provider)}
}
