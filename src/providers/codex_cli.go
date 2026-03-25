package providers

import "github.com/rinbarpen/llm-router/src/db"

type CodexCLIProviderClient struct {
	*CodeCLIProviderClient
}

func NewCodexCLIProviderClient(provider db.Provider) *CodexCLIProviderClient {
	return &CodexCLIProviderClient{CodeCLIProviderClient: NewCodeCLIProviderClient(provider)}
}
