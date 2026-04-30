package plugins

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rinbarpen/llm-router/src/config"
)

const pluginModelPrefix = "plugin:"

type PluginModelRef struct {
	PluginName string
	ModelID    string
}

type TTSVoice struct {
	ID                   string `json:"id"`
	DisplayName          string `json:"display_name,omitempty"`
	Character            string `json:"character,omitempty"`
	CharacterDisplayName string `json:"character_display_name,omitempty"`
	Timbre               string `json:"timbre,omitempty"`
	TimbreDisplayName    string `json:"timbre_display_name,omitempty"`
	Downloaded           bool   `json:"downloaded"`
	Downloading          bool   `json:"downloading,omitempty"`
	Error                string `json:"error,omitempty"`
}

type TTSPluginSummary struct {
	Name         string   `json:"name"`
	DefaultModel string   `json:"default_model,omitempty"`
	Models       []string `json:"models,omitempty"`
}

func ParsePluginModel(model string) (PluginModelRef, bool, error) {
	trimmed := strings.TrimSpace(model)
	if !strings.HasPrefix(trimmed, pluginModelPrefix) {
		return PluginModelRef{}, false, nil
	}
	raw := strings.TrimSpace(strings.TrimPrefix(trimmed, pluginModelPrefix))
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return PluginModelRef{}, true, fmt.Errorf("invalid plugin model %q, expected plugin:<plugin>/<model_id>", model)
	}
	return PluginModelRef{
		PluginName: strings.TrimSpace(parts[0]),
		ModelID:    strings.TrimSpace(parts[1]),
	}, true, nil
}

func NewTTSPlugin(name string) (TTSPlugin, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "openai_compatible":
		return &OpenAICompatibleTTSPlugin{}, nil
	case "qwen_tts":
		return &QwenTTSPlugin{}, nil
	default:
		return nil, fmt.Errorf("unsupported tts plugin %q", name)
	}
}

func NewASRPlugin(name string) (ASRPlugin, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "openai_compatible":
		return &OpenAICompatibleASRPlugin{}, nil
	case "funasr":
		return &FunASRASRPlugin{}, nil
	default:
		return nil, fmt.Errorf("unsupported asr plugin %q", name)
	}
}

func ListConfiguredTTSPlugins(cfg config.RouterModelConfig) []TTSPluginSummary {
	if cfg.Plugins == nil || len(cfg.Plugins.TTS) == 0 {
		return nil
	}
	names := make([]string, 0, len(cfg.Plugins.TTS))
	for name := range cfg.Plugins.TTS {
		names = append(names, name)
	}
	slices.Sort(names)
	out := make([]TTSPluginSummary, 0, len(names))
	for _, name := range names {
		pluginCfg := cfg.Plugins.TTS[name]
		out = append(out, TTSPluginSummary{
			Name:         name,
			DefaultModel: readString(pluginCfg, "default_model"),
			Models:       readStringSlice(pluginCfg, "models"),
		})
	}
	return out
}

func ResolveTTSPluginConfig(cfg config.RouterModelConfig, pluginName string, modelID string) (TTSPlugin, map[string]any, string, error) {
	if cfg.Plugins == nil {
		return nil, nil, "", fmt.Errorf("tts plugins are not configured")
	}
	pluginCfg, ok := cfg.Plugins.TTS[pluginName]
	if !ok {
		return nil, nil, "", fmt.Errorf("tts plugin %q is not configured", pluginName)
	}
	resolvedModelID := strings.TrimSpace(modelID)
	if resolvedModelID == "" {
		resolvedModelID = readString(pluginCfg, "default_model")
	}
	if resolvedModelID == "" {
		return nil, nil, "", fmt.Errorf("tts plugin %q requires a model_id", pluginName)
	}
	models := readStringSlice(pluginCfg, "models")
	if len(models) > 0 && !slices.Contains(models, resolvedModelID) {
		return nil, nil, "", fmt.Errorf("tts plugin %q does not support model %q", pluginName, resolvedModelID)
	}
	plugin, err := NewTTSPlugin(pluginName)
	if err != nil {
		return nil, nil, "", err
	}
	return plugin, pluginCfg, resolvedModelID, nil
}

func ResolveASRPluginConfig(cfg config.RouterModelConfig, pluginName string, modelID string) (ASRPlugin, map[string]any, string, error) {
	if cfg.Plugins == nil {
		return nil, nil, "", fmt.Errorf("asr plugins are not configured")
	}
	pluginCfg, ok := cfg.Plugins.ASR[pluginName]
	if !ok {
		return nil, nil, "", fmt.Errorf("asr plugin %q is not configured", pluginName)
	}
	resolvedModelID := strings.TrimSpace(modelID)
	if resolvedModelID == "" {
		resolvedModelID = readString(pluginCfg, "default_model")
	}
	if resolvedModelID == "" {
		return nil, nil, "", fmt.Errorf("asr plugin %q requires a model_id", pluginName)
	}
	models := readStringSlice(pluginCfg, "models")
	if len(models) > 0 && !slices.Contains(models, resolvedModelID) {
		return nil, nil, "", fmt.Errorf("asr plugin %q does not support model %q", pluginName, resolvedModelID)
	}
	plugin, err := NewASRPlugin(pluginName)
	if err != nil {
		return nil, nil, "", err
	}
	return plugin, pluginCfg, resolvedModelID, nil
}

func TTSVoicesAsMaps(voices []TTSVoice) []map[string]any {
	out := make([]map[string]any, 0, len(voices))
	for _, voice := range voices {
		item := map[string]any{
			"id":         voice.ID,
			"downloaded": voice.Downloaded,
		}
		if strings.TrimSpace(voice.DisplayName) != "" {
			item["display_name"] = voice.DisplayName
		}
		if strings.TrimSpace(voice.Character) != "" {
			item["character"] = voice.Character
		}
		if strings.TrimSpace(voice.CharacterDisplayName) != "" {
			item["character_display_name"] = voice.CharacterDisplayName
		}
		if strings.TrimSpace(voice.Timbre) != "" {
			item["timbre"] = voice.Timbre
		}
		if strings.TrimSpace(voice.TimbreDisplayName) != "" {
			item["timbre_display_name"] = voice.TimbreDisplayName
		}
		if voice.Downloading {
			item["downloading"] = true
		}
		if strings.TrimSpace(voice.Error) != "" {
			item["error"] = voice.Error
		}
		out = append(out, item)
	}
	return out
}

type TTSPlugin interface {
	SynthesizeSpeech(ctx context.Context, modelID string, payload map[string]any, config map[string]any) ([]byte, string, error)
	ListVoices(ctx context.Context, modelID string, config map[string]any) ([]TTSVoice, error)
}

func readStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	raw, ok := m[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}
