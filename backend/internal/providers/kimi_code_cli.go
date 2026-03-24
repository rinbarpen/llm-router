package providers

import "github.com/rinbarpen/llm-router/backend/internal/db"

type KimiCodeCLIProviderClient struct {
	*CodeCLIProviderClient
}

func NewKimiCodeCLIProviderClient(provider db.Provider) *KimiCodeCLIProviderClient {
	return &KimiCodeCLIProviderClient{CodeCLIProviderClient: NewCodeCLIProviderClient(provider)}
}
