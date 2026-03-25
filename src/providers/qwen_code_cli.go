package providers

import "github.com/rinbarpen/llm-router/src/db"

type QwenCodeCLIProviderClient struct {
	*CodeCLIProviderClient
}

func NewQwenCodeCLIProviderClient(provider db.Provider) *QwenCodeCLIProviderClient {
	return &QwenCodeCLIProviderClient{CodeCLIProviderClient: NewCodeCLIProviderClient(provider)}
}
