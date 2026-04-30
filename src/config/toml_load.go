package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

var reservedTOMLRoots = map[string]struct{}{
	"providers": {}, "models": {}, "api_keys": {},
	"server": {}, "monitor": {}, "logging": {}, "routing": {}, "plugins": {}, "model_updates": {},
}

// LoadRouterModelConfigFromTOML parses router.toml (including nested provider blocks with models, like Python load_model_config).
func LoadRouterModelConfigFromTOML(path string) (RouterModelConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return RouterModelConfig{}, fmt.Errorf("read %s: %w", path, err)
	}
	var data map[string]any
	if err := toml.Unmarshal(raw, &data); err != nil {
		return RouterModelConfig{}, fmt.Errorf("parse toml %s: %w", path, err)
	}

	var cfg RouterModelConfig

	if err := decodeSliceMaps(data["providers"], defaultActiveInMap); err != nil {
		return RouterModelConfig{}, fmt.Errorf("providers: %w", err)
	}
	if err := marshalUnmarshal(data["providers"], &cfg.Providers); err != nil {
		return RouterModelConfig{}, fmt.Errorf("providers decode: %w", err)
	}

	allModels := collectModelTables(data)
	if err := decodeSliceMaps(anySlice(allModels), defaultActiveInMap); err != nil {
		return RouterModelConfig{}, fmt.Errorf("models: %w", err)
	}
	if err := marshalUnmarshal(anySlice(allModels), &cfg.Models); err != nil {
		return RouterModelConfig{}, fmt.Errorf("models decode: %w", err)
	}

	if err := decodeSliceMaps(data["api_keys"], defaultActiveInMap); err != nil {
		return RouterModelConfig{}, fmt.Errorf("api_keys: %w", err)
	}
	if err := marshalUnmarshal(data["api_keys"], &cfg.APIKeys); err != nil {
		return RouterModelConfig{}, fmt.Errorf("api_keys decode: %w", err)
	}
	if err := marshalUnmarshal(data["server"], &cfg.Server); err != nil {
		return RouterModelConfig{}, fmt.Errorf("server decode: %w", err)
	}
	if err := marshalUnmarshal(data["monitor"], &cfg.Monitor); err != nil {
		return RouterModelConfig{}, fmt.Errorf("monitor decode: %w", err)
	}
	if err := marshalUnmarshal(data["logging"], &cfg.Logging); err != nil {
		return RouterModelConfig{}, fmt.Errorf("logging decode: %w", err)
	}
	if err := marshalUnmarshal(data["routing"], &cfg.Routing); err != nil {
		return RouterModelConfig{}, fmt.Errorf("routing decode: %w", err)
	}
	if err := marshalUnmarshal(data["plugins"], &cfg.Plugins); err != nil {
		return RouterModelConfig{}, fmt.Errorf("plugins decode: %w", err)
	}
	if err := marshalUnmarshal(data["model_updates"], &cfg.ModelUpdates); err != nil {
		return RouterModelConfig{}, fmt.Errorf("model_updates decode: %w", err)
	}

	cfg.Normalize()
	return cfg, nil
}

func collectModelTables(data map[string]any) []any {
	var out []any
	if v, ok := data["models"]; ok {
		if arr, ok := v.([]any); ok {
			out = append(out, arr...)
		}
	}
	for k, v := range data {
		if _, skip := reservedTOMLRoots[k]; skip {
			continue
		}
		obj, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if mm, ok := obj["models"]; ok {
			if arr, ok := mm.([]any); ok {
				out = append(out, arr...)
			}
		}
	}
	return out
}

func anySlice(items []any) any {
	if len(items) == 0 {
		return nil
	}
	return items
}

func decodeSliceMaps(v any, patch func(map[string]any)) error {
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return fmt.Errorf("expected array, got %T", v)
	}
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		patch(m)
	}
	return nil
}

func defaultActiveInMap(m map[string]any) {
	if _, ok := m["is_active"]; !ok {
		m["is_active"] = true
	}
}

func marshalUnmarshal(v any, target any) error {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}
