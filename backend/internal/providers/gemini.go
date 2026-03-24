package providers

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/rinbarpen/llm-router/backend/internal/db"
)

// GeminiProviderClient invokes Gemini native generateContent endpoint.
type GeminiProviderClient struct {
	HTTPProviderBase
}

func NewGeminiProviderClient(provider db.Provider) *GeminiProviderClient {
	return &GeminiProviderClient{HTTPProviderBase: NewHTTPProviderBase(provider)}
}

func (c *GeminiProviderClient) Invoke(ctx context.Context, model db.Model, payload map[string]any) (map[string]any, error) {
	contents, ok := payload["contents"]
	if !ok {
		messages := parseMessageLikeInput(payload)
		if len(messages) == 0 {
			return nil, &ProviderError{Message: "contents or messages is required"}
		}
		contents = openAIMessagesToGeminiContents(messages)
	}

	body := map[string]any{"contents": contents}
	if gc, ok := payload["generationConfig"]; ok {
		body["generationConfig"] = gc
	}

	for k, v := range mergeRequestParameters(model, payload) {
		switch k {
		case "contents", "messages", "prompt", "model", "remote_identifier_override", "generationConfig":
			continue
		default:
			body[k] = v
		}
	}

	apiKey := resolveAPIKey(c.Provider)
	if apiKey == "" {
		return nil, &ProviderError{Message: "gemini api_key is required"}
	}

	endpoint := c.buildEndpoint(model, apiKey)
	return c.doJSON(ctx, "POST", endpoint, body, map[string]string{})
}

func (c *GeminiProviderClient) buildEndpoint(model db.Model, apiKey string) string {
	base := "https://generativelanguage.googleapis.com"
	if c.Provider.BaseURL != nil && strings.TrimSpace(*c.Provider.BaseURL) != "" {
		base = strings.TrimSpace(*c.Provider.BaseURL)
	} else if raw, ok := c.Provider.Settings["base_url"].(string); ok && strings.TrimSpace(raw) != "" {
		base = strings.TrimSpace(raw)
	}
	endpointTemplate := "/v1beta/models/{model}:generateContent"
	if raw, ok := c.Provider.Settings["endpoint_template"].(string); ok && strings.TrimSpace(raw) != "" {
		endpointTemplate = strings.TrimSpace(raw)
	}
	modelID := resolveModelIdentifier(model, nil)
	if modelID == "" {
		modelID = model.Name
	}
	endpoint := strings.ReplaceAll(endpointTemplate, "{model}", modelID)
	query := url.Values{}
	query.Set("key", apiKey)
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(endpoint, "/") + "?" + query.Encode()
}

func openAIMessagesToGeminiContents(messages []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		role := sanitizeString(message["role"])
		if role == "" || role == "system" {
			role = "user"
		}
		if role != "user" {
			role = "model"
		}
		content := message["content"]
		text := fmt.Sprintf("%v", content)
		if s, ok := content.(string); ok {
			text = s
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		out = append(out, map[string]any{
			"role":  role,
			"parts": []map[string]any{{"text": text}},
		})
	}
	return out
}
