package providers

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/rinbarpen/llm-router/backend/internal/db"
)

// OpenAICompatibleProviderClient supports OpenAI-compatible chat endpoints.
type OpenAICompatibleProviderClient struct {
	HTTPProviderBase
}

func NewOpenAICompatibleProviderClient(provider db.Provider) *OpenAICompatibleProviderClient {
	return &OpenAICompatibleProviderClient{HTTPProviderBase: NewHTTPProviderBase(provider)}
}

var defaultBaseURLs = map[db.ProviderType]string{
	db.ProviderTypeOpenAI:      "https://api.openai.com/v1",
	db.ProviderTypeGrok:        "https://api.x.ai",
	db.ProviderTypeDeepseek:    "https://api.deepseek.com",
	db.ProviderTypeQwen:        "https://dashscope.aliyuncs.com",
	db.ProviderTypeKimi:        "https://api.moonshot.cn",
	db.ProviderTypeGLM:         "https://open.bigmodel.cn/api/paas/v4",
	db.ProviderTypeOpenRouter:  "https://openrouter.ai/api",
	db.ProviderTypeMiniMax:     "https://api.minimax.chat/v1",
	db.ProviderTypeDoubao:      "https://ark.cn-beijing.volces.com/api/v3",
	db.ProviderTypeGroq:        "https://api.groq.com/openai/v1",
	db.ProviderTypeSiliconFlow: "https://api.siliconflow.cn/v1",
	db.ProviderTypeAIHubMix:    "https://aihubmix.com/v1",
	db.ProviderTypeVolcengine:  "https://ark.cn-beijing.volces.com/api/v3",
}

var endpointOverrides = map[db.ProviderType]string{
	db.ProviderTypeQwen:       "/compatible-mode/v1/chat/completions",
	db.ProviderTypeGLM:        "/chat/completions",
	db.ProviderTypeMiniMax:    "/text/chatcompletion_v2",
	db.ProviderTypeDoubao:     "/chat/completions",
	db.ProviderTypeVolcengine: "/chat/completions",
}

func (c *OpenAICompatibleProviderClient) Invoke(ctx context.Context, model db.Model, payload map[string]any) (map[string]any, error) {
	messages := parseMessageLikeInput(payload)
	if len(messages) == 0 {
		return nil, &ProviderError{Message: "prompt or messages is required"}
	}

	body := mergeRequestParameters(model, payload)
	body["messages"] = messages
	body["model"] = resolveModelIdentifier(model, payload)
	delete(body, "prompt")

	endpoint := c.buildEndpoint()
	headers := map[string]string{}
	if key := resolveAPIKey(c.Provider); key != "" {
		headers[c.authHeaderName()] = strings.TrimSpace(c.authScheme() + " " + key)
	}

	return c.doJSON(ctx, "POST", endpoint, body, headers)
}

func (c *OpenAICompatibleProviderClient) buildEndpoint() string {
	base := ""
	if c.Provider.BaseURL != nil {
		base = strings.TrimSpace(*c.Provider.BaseURL)
	}
	if base == "" {
		if raw, ok := c.Provider.Settings["base_url"].(string); ok {
			base = strings.TrimSpace(raw)
		}
	}
	if base == "" {
		base = defaultBaseURLs[c.Provider.Type]
	}
	if base == "" {
		base = "https://api.openai.com/v1"
	}

	endpoint := "/v1/chat/completions"
	if raw, ok := c.Provider.Settings["endpoint"].(string); ok && strings.TrimSpace(raw) != "" {
		endpoint = strings.TrimSpace(raw)
	} else if overridden, ok := endpointOverrides[c.Provider.Type]; ok {
		endpoint = overridden
	}

	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	base = strings.TrimRight(base, "/")
	endpoint = "/" + strings.TrimLeft(endpoint, "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	if strings.HasSuffix(base, "/v1") && endpoint == "/v1/chat/completions" {
		return base + "/chat/completions"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" {
		return base + endpoint
	}
	return base + endpoint
}

func (c *OpenAICompatibleProviderClient) authHeaderName() string {
	if raw, ok := c.Provider.Settings["auth_header"].(string); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return "Authorization"
}

func (c *OpenAICompatibleProviderClient) authScheme() string {
	if raw, ok := c.Provider.Settings["auth_scheme"].(string); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return "Bearer"
}

func newOpenAICompatibleAlias(provider db.Provider) BaseProviderClient {
	return NewOpenAICompatibleProviderClient(provider)
}

func sanitizeString(value any) string {
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func requiredString(value any, field string) (string, error) {
	v := sanitizeString(value)
	if v == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	return v, nil
}
