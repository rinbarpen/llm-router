package providers

import (
	"context"
	"strings"

	"github.com/rinbarpen/llm-router/backend/internal/db"
)

// RemoteHTTPProviderClient posts raw payload to configured custom endpoint.
type RemoteHTTPProviderClient struct {
	HTTPProviderBase
}

func NewRemoteHTTPProviderClient(provider db.Provider) *RemoteHTTPProviderClient {
	return &RemoteHTTPProviderClient{HTTPProviderBase: NewHTTPProviderBase(provider)}
}

func (c *RemoteHTTPProviderClient) Invoke(ctx context.Context, model db.Model, payload map[string]any) (map[string]any, error) {
	endpoint := ""
	if raw, ok := c.Provider.Settings["endpoint"].(string); ok {
		endpoint = strings.TrimSpace(raw)
	}
	if endpoint == "" {
		if c.Provider.BaseURL != nil {
			endpoint = strings.TrimSpace(*c.Provider.BaseURL)
		}
	}
	if endpoint == "" {
		return nil, &ProviderError{Message: "remote_http endpoint is required"}
	}
	body := mergeRequestParameters(model, payload)
	if _, ok := body["model"]; !ok {
		body["model"] = resolveModelIdentifier(model, payload)
	}
	headers := map[string]string{}
	if key := resolveAPIKey(c.Provider); key != "" {
		headers["Authorization"] = "Bearer " + key
	}
	return c.doJSON(ctx, "POST", endpoint, body, headers)
}
