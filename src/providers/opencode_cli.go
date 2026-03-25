package providers

import "github.com/rinbarpen/llm-router/src/db"

type OpenCodeCLIProviderClient struct {
	*CodeCLIProviderClient
}

func NewOpenCodeCLIProviderClient(provider db.Provider) *OpenCodeCLIProviderClient {
	return &OpenCodeCLIProviderClient{CodeCLIProviderClient: NewCodeCLIProviderClient(provider)}
}
