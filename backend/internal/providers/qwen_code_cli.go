package providers

import "github.com/rinbarpen/llm-router/backend/internal/db"

type QwenCodeCLIProviderClient struct {
	*CodeCLIProviderClient
}

func NewQwenCodeCLIProviderClient(provider db.Provider) *QwenCodeCLIProviderClient {
	return &QwenCodeCLIProviderClient{CodeCLIProviderClient: NewCodeCLIProviderClient(provider)}
}
