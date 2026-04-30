package services

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rinbarpen/llm-router/src/config"
	"github.com/rinbarpen/llm-router/src/plugins"
)

func loadPluginRuntimeConfig() (config.RouterModelConfig, error) {
	path := strings.TrimSpace(os.Getenv("LLM_ROUTER_MODEL_CONFIG"))
	if path == "" {
		path = strings.TrimSpace(os.Getenv("LLM_ROUTER_MODEL_CONFIG_FILE"))
	}
	if path == "" {
		path = "router.toml"
	}
	resolved, err := config.ResolveModelConfigPath(path)
	if err != nil {
		return config.RouterModelConfig{}, err
	}
	return config.LoadRouterModelConfigFromTOML(resolved)
}

func normalizeSpeechPayload(payload map[string]any) map[string]any {
	body := map[string]any{}
	for k, v := range payload {
		body[k] = v
	}
	voice := strings.TrimSpace(fmt.Sprintf("%v", body["voice"]))
	role := strings.TrimSpace(fmt.Sprintf("%v", body["role"]))
	switch {
	case voice != "" && voice != "<nil>":
	case role != "" && role != "<nil>":
		body["voice"] = role
	}
	delete(body, "role")
	return body
}

func trySynthesizeWithPlugin(ctx context.Context, payload map[string]any) ([]byte, string, bool, error) {
	modelName, _ := payload["model"].(string)
	ref, ok, err := plugins.ParsePluginModel(modelName)
	if err != nil {
		return nil, "", true, err
	}
	if !ok {
		return nil, "", false, nil
	}
	cfg, err := loadPluginRuntimeConfig()
	if err != nil {
		return nil, "", true, err
	}
	plugin, pluginCfg, modelID, err := plugins.ResolveTTSPluginConfig(cfg, ref.PluginName, ref.ModelID)
	if err != nil {
		return nil, "", true, err
	}
	audio, contentType, err := plugin.SynthesizeSpeech(ctx, modelID, normalizeSpeechPayload(payload), pluginCfg)
	return audio, contentType, true, err
}

func tryTranscribeWithPlugin(ctx context.Context, payload map[string]any, fileData []byte, filename string, mimeType string, translate bool) (map[string]any, bool, error) {
	modelName, _ := payload["model"].(string)
	ref, ok, err := plugins.ParsePluginModel(modelName)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		return nil, false, nil
	}
	cfg, err := loadPluginRuntimeConfig()
	if err != nil {
		return nil, true, err
	}
	plugin, pluginCfg, modelID, err := plugins.ResolveASRPluginConfig(cfg, ref.PluginName, ref.ModelID)
	if err != nil {
		return nil, true, err
	}
	body := map[string]any{}
	for k, v := range payload {
		if k == "model" {
			continue
		}
		body[k] = v
	}
	if translate {
		out, err := plugin.TranslateAudio(ctx, modelID, fileData, filename, mimeType, body, pluginCfg)
		return out, true, err
	}
	out, err := plugin.TranscribeAudio(ctx, modelID, fileData, filename, mimeType, body, pluginCfg)
	return out, true, err
}

func (s *CatalogService) ListTTSPlugins(_ context.Context) ([]map[string]any, error) {
	cfg, err := loadPluginRuntimeConfig()
	if err != nil {
		return nil, err
	}
	items := plugins.ListConfiguredTTSPlugins(cfg)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"name":          item.Name,
			"default_model": item.DefaultModel,
			"models":        item.Models,
		})
	}
	return out, nil
}

func (s *CatalogService) ListTTSPluginVoices(ctx context.Context, pluginName string, modelID string) ([]map[string]any, error) {
	cfg, err := loadPluginRuntimeConfig()
	if err != nil {
		return nil, err
	}
	plugin, pluginCfg, resolvedModelID, err := plugins.ResolveTTSPluginConfig(cfg, pluginName, modelID)
	if err != nil {
		return nil, err
	}
	voices, err := plugin.ListVoices(ctx, resolvedModelID, pluginCfg)
	if err != nil {
		return nil, err
	}
	return plugins.TTSVoicesAsMaps(voices), nil
}
