package providers

import (
	"context"
	"strings"

	"github.com/rinbarpen/llm-router/src/db"
)

// AnthropicProviderClient invokes Claude native messages endpoint.
type AnthropicProviderClient struct {
	HTTPProviderBase
}

func NewAnthropicProviderClient(provider db.Provider) *AnthropicProviderClient {
	return &AnthropicProviderClient{HTTPProviderBase: NewHTTPProviderBase(provider)}
}

func (c *AnthropicProviderClient) Invoke(ctx context.Context, model db.Model, payload map[string]any) (map[string]any, error) {
	messages := payload["messages"]
	if messages == nil {
		openAIMessages := parseMessageLikeInput(payload)
		if len(openAIMessages) == 0 {
			return nil, &ProviderError{Message: "messages is required"}
		}
		messages = openAIMessagesToClaudeMessages(openAIMessages)
	}

	body := mergeRequestParameters(model, payload)
	body["model"] = resolveModelIdentifier(model, payload)
	body["messages"] = messages
	if _, ok := body["max_tokens"]; !ok {
		body["max_tokens"] = 1024
	}

	for _, transient := range []string{"prompt", "remote_identifier_override"} {
		delete(body, transient)
	}

	apiKey := resolveAPIKey(c.Provider)
	if apiKey == "" {
		return nil, &ProviderError{Message: "claude api_key is required"}
	}

	headers := map[string]string{
		"x-api-key":         apiKey,
		"anthropic-version": c.version(),
	}
	endpoint := c.buildEndpoint()
	return c.doJSON(ctx, "POST", endpoint, body, headers)
}

func (c *AnthropicProviderClient) version() string {
	if raw, ok := c.Provider.Settings["anthropic_version"].(string); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return "2023-06-01"
}

func (c *AnthropicProviderClient) buildEndpoint() string {
	base := "https://api.anthropic.com"
	if c.Provider.BaseURL != nil && strings.TrimSpace(*c.Provider.BaseURL) != "" {
		base = strings.TrimSpace(*c.Provider.BaseURL)
	} else if raw, ok := c.Provider.Settings["base_url"].(string); ok && strings.TrimSpace(raw) != "" {
		base = strings.TrimSpace(raw)
	}
	endpoint := "/v1/messages"
	if raw, ok := c.Provider.Settings["endpoint"].(string); ok && strings.TrimSpace(raw) != "" {
		endpoint = strings.TrimSpace(raw)
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(endpoint, "/")
}

func openAIMessagesToClaudeMessages(messages []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		role := sanitizeString(message["role"])
		if role == "" || role == "system" {
			role = "user"
		}
		if role != "user" {
			role = "assistant"
		}
		content := sanitizeString(message["content"])
		if content == "" {
			continue
		}
		out = append(out, map[string]any{
			"role": role,
			"content": []map[string]any{{
				"type": "text",
				"text": content,
			}},
		})
	}
	return out
}
