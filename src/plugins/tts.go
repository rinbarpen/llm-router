package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"
)

// OpenAICompatibleTTSPlugin forwards to /v1/audio/speech style endpoints.
type OpenAICompatibleTTSPlugin struct{}

func (p *OpenAICompatibleTTSPlugin) SynthesizeSpeech(ctx context.Context, modelID string, payload map[string]any, config map[string]any) ([]byte, string, error) {
	baseURL := readString(config, "base_url")
	if baseURL == "" {
		return nil, "", fmt.Errorf("tts plugin openai_compatible requires base_url")
	}
	endpoint := readStringDefault(config, "audio_speech_endpoint", "/v1/audio/speech")

	body := cloneMap(payload)
	body["model"] = modelID
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("marshal tts payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(baseURL, endpoint), bytes.NewReader(raw))
	if err != nil {
		return nil, "", fmt.Errorf("build tts request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if key := resolveSecret(config, "api_key", "api_key_env"); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := (&http.Client{Timeout: readTimeout(config, 60*time.Second)}).Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("invoke tts: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("tts upstream failed: %d %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	mediaType := "audio/mpeg"
	if ct := strings.TrimSpace(resp.Header.Get("Content-Type")); ct != "" {
		mediaType, _, _ = mime.ParseMediaType(ct)
		if mediaType == "" {
			mediaType = "audio/mpeg"
		}
	}
	return data, mediaType, nil
}

func (p *OpenAICompatibleTTSPlugin) ListVoices(_ context.Context, _ string, _ map[string]any) ([]TTSVoice, error) {
	return nil, fmt.Errorf("tts plugin openai_compatible does not support voice listing")
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func readString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if raw, ok := m[key]; ok {
		if s, ok := raw.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func readStringDefault(m map[string]any, key string, fallback string) string {
	if v := readString(m, key); v != "" {
		return v
	}
	return fallback
}

func readTimeout(config map[string]any, fallback time.Duration) time.Duration {
	if config != nil {
		if raw, ok := config["timeout"]; ok {
			switch v := raw.(type) {
			case float64:
				if v > 0 {
					return time.Duration(v * float64(time.Second))
				}
			case int:
				if v > 0 {
					return time.Duration(v) * time.Second
				}
			case int64:
				if v > 0 {
					return time.Duration(v) * time.Second
				}
			}
		}
	}
	return fallback
}

func resolveSecret(config map[string]any, valueKey, envKey string) string {
	if v := readString(config, valueKey); v != "" {
		return v
	}
	if envName := readString(config, envKey); envName != "" {
		return strings.TrimSpace(os.Getenv(envName))
	}
	return ""
}

func joinURL(baseURL, endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(endpoint, "/")
}
