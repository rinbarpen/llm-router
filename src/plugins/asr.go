package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// ASRPlugin defines audio transcription capability.
type ASRPlugin interface {
	TranscribeAudio(ctx context.Context, modelID string, data []byte, filename string, mimeType string, extraPayload map[string]any, config map[string]any) (map[string]any, error)
	TranslateAudio(ctx context.Context, modelID string, data []byte, filename string, mimeType string, extraPayload map[string]any, config map[string]any) (map[string]any, error)
}

// OpenAICompatibleASRPlugin forwards ASR to OpenAI-compatible endpoints.
type OpenAICompatibleASRPlugin struct{}

func (p *OpenAICompatibleASRPlugin) TranscribeAudio(ctx context.Context, modelID string, data []byte, filename string, mimeType string, extraPayload map[string]any, config map[string]any) (map[string]any, error) {
	return p.audioRequest(ctx, modelID, data, filename, mimeType, extraPayload, config, "audio_transcriptions_endpoint", "/v1/audio/transcriptions")
}

func (p *OpenAICompatibleASRPlugin) TranslateAudio(ctx context.Context, modelID string, data []byte, filename string, mimeType string, extraPayload map[string]any, config map[string]any) (map[string]any, error) {
	return p.audioRequest(ctx, modelID, data, filename, mimeType, extraPayload, config, "audio_translations_endpoint", "/v1/audio/translations")
}

func (p *OpenAICompatibleASRPlugin) audioRequest(ctx context.Context, modelID string, data []byte, filename string, mimeType string, extraPayload map[string]any, config map[string]any, endpointKey string, fallbackEndpoint string) (map[string]any, error) {
	baseURL := readString(config, "base_url")
	if baseURL == "" {
		return nil, fmt.Errorf("asr plugin openai_compatible requires base_url")
	}
	endpoint := readStringDefault(config, endpointKey, fallbackEndpoint)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("model", modelID)
	for k, v := range extraPayload {
		if v == nil {
			continue
		}
		_ = writer.WriteField(k, fmt.Sprintf("%v", v))
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create multipart file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write audio payload: %w", err)
	}
	_ = writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(baseURL, endpoint), body)
	if err != nil {
		return nil, fmt.Errorf("build asr request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if key := resolveSecret(config, "api_key", "api_key_env"); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if strings.TrimSpace(mimeType) != "" {
		req.Header.Set("X-File-Mime-Type", mimeType)
	}

	resp, err := (&http.Client{Timeout: readTimeout(config, 60*time.Second)}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoke asr: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("asr upstream failed: %d %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	out := map[string]any{}
	if len(respBody) > 0 && json.Valid(respBody) {
		_ = json.Unmarshal(respBody, &out)
	}
	if len(out) == 0 {
		out["text"] = strings.TrimSpace(string(respBody))
	}
	return out, nil
}
