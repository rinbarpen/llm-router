package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rinbarpen/llm-router/src/db"
)

// ErrProviderUnsupported marks provider capability not implemented yet.
var ErrProviderUnsupported = errors.New("provider capability not supported")

// ProviderError carries the upstream status and message.
type ProviderError struct {
	StatusCode int
	Message    string
}

func (e *ProviderError) Error() string {
	if e == nil {
		return "provider error"
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("provider error (%d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("provider error: %s", e.Message)
}

// BaseProviderClient defines the unified provider invoke contract.
type BaseProviderClient interface {
	Invoke(ctx context.Context, model db.Model, payload map[string]any) (map[string]any, error)
}

// HTTPProviderBase provides shared HTTP helpers for provider adapters.
type HTTPProviderBase struct {
	Provider db.Provider
	Client   *http.Client
}

func NewHTTPProviderBase(provider db.Provider) HTTPProviderBase {
	timeout := providerTimeout(provider)
	return HTTPProviderBase{
		Provider: provider,
		Client: &http.Client{
			Timeout: timeout,
		},
	}
}

func providerTimeout(provider db.Provider) time.Duration {
	if provider.Settings != nil {
		if raw, ok := provider.Settings["timeout"]; ok {
			switch v := raw.(type) {
			case float64:
				if v > 0 {
					return time.Duration(v * float64(time.Second))
				}
			case int64:
				if v > 0 {
					return time.Duration(v) * time.Second
				}
			case int:
				if v > 0 {
					return time.Duration(v) * time.Second
				}
			case string:
				if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && f > 0 {
					return time.Duration(f * float64(time.Second))
				}
			}
		}
	}
	return 90 * time.Second
}

func (b *HTTPProviderBase) doJSON(ctx context.Context, method, url string, body any, headers map[string]string) (map[string]any, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		req.Header.Set(k, v)
	}

	resp, err := b.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoke provider: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(respBody) > 0 && json.Valid(respBody) {
		_ = json.Unmarshal(respBody, &out)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if detail, ok := out["error"]; ok {
			msg = fmt.Sprintf("%v", detail)
		}
		return nil, &ProviderError{StatusCode: resp.StatusCode, Message: msg}
	}

	if len(out) == 0 {
		return nil, &ProviderError{StatusCode: resp.StatusCode, Message: "provider returned non-json payload"}
	}
	return out, nil
}

func resolveAPIKey(provider db.Provider) string {
	if provider.APIKey != nil && strings.TrimSpace(*provider.APIKey) != "" {
		return strings.TrimSpace(*provider.APIKey)
	}
	if provider.Settings != nil {
		if raw, ok := provider.Settings["api_key"]; ok {
			if key, ok := raw.(string); ok {
				if strings.TrimSpace(key) != "" {
					return strings.TrimSpace(key)
				}
			}
		}
		if raw, ok := provider.Settings["api_key_env"]; ok {
			if envName, ok := raw.(string); ok && strings.TrimSpace(envName) != "" {
				if key := strings.TrimSpace(os.Getenv(strings.TrimSpace(envName))); key != "" {
					return key
				}
			}
		}
	}
	return ""
}

func resolveModelIdentifier(model db.Model, payload map[string]any) string {
	if raw, ok := payload["remote_identifier_override"]; ok {
		if v, ok := raw.(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if model.RemoteIdentifier != nil && strings.TrimSpace(*model.RemoteIdentifier) != "" {
		return strings.TrimSpace(*model.RemoteIdentifier)
	}
	if model.Config != nil {
		if raw, ok := model.Config["model"]; ok {
			if v, ok := raw.(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return model.Name
}

func mergeRequestParameters(model db.Model, payload map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range model.DefaultParams {
		out[k] = v
	}
	for k, v := range payload {
		out[k] = v
	}
	return out
}

func parseMessageLikeInput(payload map[string]any) []map[string]any {
	messages := make([]map[string]any, 0)
	if raw, ok := payload["messages"].([]any); ok {
		for _, item := range raw {
			if m, ok := item.(map[string]any); ok {
				messages = append(messages, m)
			}
		}
	}
	if raw, ok := payload["messages"].([]map[string]any); ok {
		messages = append(messages, raw...)
	}
	if prompt, ok := payload["prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
		messages = append(messages, map[string]any{"role": "user", "content": prompt})
	}
	return messages
}
