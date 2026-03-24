package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var dataURLPattern = regexp.MustCompile(`^data:([^;]+);base64,(.+)$`)

func normalizeProviderName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	normalized := strings.ToLower(trimmed)
	switch normalized {
	case "claude_code":
		return "claude_code_cli"
	case "opencode":
		return "opencode_cli"
	case "kimi_code":
		return "kimi_code_cli"
	case "qwen_code":
		return "qwen_code_cli"
	default:
		return trimmed
	}
}

func normalizeClaudeProviderName(name string) string {
	return normalizeProviderName(name)
}

// normalizeMultimodalContent converts OpenAI multimodal message content into a normalized shape:
// - string -> string
// - [{type:text},{type:image_url}] -> []map[string]any
func normalizeMultimodalContent(content any) any {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			switch part["type"] {
			case "text":
				text, _ := part["text"].(string)
				if strings.TrimSpace(text) != "" {
					out = append(out, map[string]any{"type": "text", "text": text})
				}
			case "image_url":
				url := ""
				switch iv := part["image_url"].(type) {
				case map[string]any:
					url, _ = iv["url"].(string)
				case string:
					url = iv
				}
				if strings.TrimSpace(url) != "" {
					out = append(out, map[string]any{"type": "image_url", "url": url})
				}
			}
		}
		if len(out) == 1 && out[0]["type"] == "text" {
			if text, ok := out[0]["text"].(string); ok {
				return text
			}
		}
		if len(out) == 0 {
			return ""
		}
		return out
	default:
		return fmt.Sprintf("%v", content)
	}
}

func extractBase64FromDataURL(raw string) (mimeType string, data string) {
	matches := dataURLPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(matches) != 3 {
		return "image/png", ""
	}
	return strings.TrimSpace(matches[1]), strings.TrimSpace(matches[2])
}

func validateBase64Payload(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	_, err := base64.StdEncoding.DecodeString(raw)
	return err == nil
}

func readJSONBody(r *http.Request, allowEmpty bool) (map[string]any, error) {
	defer r.Body.Close()
	limited := io.LimitReader(r.Body, 4<<20)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		if allowEmpty {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("request body cannot be empty")
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("invalid json body: %w", err)
	}
	obj, ok := payload.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("request body must be a json object")
	}
	return obj, nil
}
