package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rinbarpen/llm-router/src/schemas"
	"github.com/rinbarpen/llm-router/src/services"
)

type authContextData struct {
	APIKey   schemas.APIKey
	Policy   services.APIKeyPolicy
	AuthType string
}

type authContextKeyType string

const authContextKey authContextKeyType = "llm_router_auth_context"

func withAuthContext(ctx context.Context, data authContextData) context.Context {
	return context.WithValue(ctx, authContextKey, data)
}

func getAuthContext(ctx context.Context) (authContextData, bool) {
	v := ctx.Value(authContextKey)
	if v == nil {
		return authContextData{}, false
	}
	item, ok := v.(authContextData)
	return item, ok
}

func checkAPIKeyLifecycle(item schemas.APIKey) error {
	if item.ExpiresAt != nil && time.Now().UTC().After(item.ExpiresAt.UTC()) {
		return fmt.Errorf("api key expired")
	}
	return nil
}

func resolveClientIP(req *http.Request) string {
	if req == nil {
		return ""
	}
	host := strings.TrimSpace(req.RemoteAddr)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = strings.TrimSpace(h)
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return ""
}

func isIPAllowed(allowlist []string, req *http.Request) bool {
	allowlist = normalizeStringList(allowlist)
	if len(allowlist) == 0 {
		return true
	}
	clientIP := resolveClientIP(req)
	if clientIP == "" {
		return false
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}
	for _, item := range allowlist {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			if _, cidr, err := net.ParseCIDR(item); err == nil && cidr.Contains(ip) {
				return true
			}
			continue
		}
		if raw := net.ParseIP(item); raw != nil && raw.Equal(ip) {
			return true
		}
	}
	return false
}

func normalizeStringList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if v := strings.TrimSpace(item); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func extractProviderModel(providerHint string, model string) (string, string) {
	provider := strings.TrimSpace(providerHint)
	name := strings.TrimSpace(model)
	if parts := strings.SplitN(name, "/", 2); len(parts) == 2 {
		if provider == "" {
			provider = strings.TrimSpace(parts[0])
		}
		name = strings.TrimSpace(parts[1])
	}
	return provider, name
}

func applyParameterLimits(payload map[string]any, limits map[string]any) {
	if payload == nil || limits == nil {
		return
	}
	for _, key := range []string{"max_tokens", "temperature", "top_p", "frequency_penalty", "presence_penalty"} {
		limit, ok := numericValue(limits[key])
		if !ok {
			continue
		}
		if current, hasCurrent := numericValue(payload[key]); hasCurrent && current > limit {
			payload[key] = preserveType(payload[key], limit)
		}
	}
}

func numericValue(v any) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case float32:
		return float64(t), true
	case float64:
		return t, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func preserveType(raw any, limited float64) any {
	switch raw.(type) {
	case int:
		return int(limited)
	case int32:
		return int32(limited)
	case int64:
		return int64(limited)
	case float32:
		return float32(limited)
	default:
		return limited
	}
}
