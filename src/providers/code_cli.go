package providers

import (
	"context"
	"fmt"

	"github.com/rinbarpen/llm-router/src/db"
)

// CodeCLIProviderClient is a base adapter for local CLI-based code assistants.
type CodeCLIProviderClient struct {
	Provider db.Provider
}

func NewCodeCLIProviderClient(provider db.Provider) *CodeCLIProviderClient {
	return &CodeCLIProviderClient{Provider: provider}
}

func (c *CodeCLIProviderClient) Invoke(_ context.Context, _ db.Model, _ map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("%w: cli provider %s is not wired in Go backend yet", ErrProviderUnsupported, c.Provider.Type)
}
