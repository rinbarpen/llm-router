package providers

import "github.com/rinbarpen/llm-router/src/db"

type KimiCodeCLIProviderClient struct {
	*CodeCLIProviderClient
}

func NewKimiCodeCLIProviderClient(provider db.Provider) *KimiCodeCLIProviderClient {
	return &KimiCodeCLIProviderClient{CodeCLIProviderClient: NewCodeCLIProviderClient(provider)}
}
