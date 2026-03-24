package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/rinbarpen/llm-router/backend/internal/schemas"
)

// RoutingError indicates model/provider selection failure.
type RoutingError struct {
	Message string
}

func (e *RoutingError) Error() string { return e.Message }

// RouterCatalog keeps RouterEngine independent from db implementation.
type RouterCatalog interface {
	ListModels(ctx context.Context) ([]schemas.Model, error)
	ListModelsByProvider(ctx context.Context, providerName string) ([]schemas.Model, error)
	GetModelByProviderAndName(ctx context.Context, providerName string, modelName string) (schemas.Model, error)
	OpenAIChatCompletions(ctx context.Context, providerHint string, payload map[string]any) (map[string]any, error)
}

// RouterEngine mirrors python route-by-tags/route-by-name logic for Go paths.
type RouterEngine struct {
	catalog     RouterCatalog
	rateLimiter *RateLimiterManager
}

func NewRouterEngine(catalog RouterCatalog, limiter *RateLimiterManager) *RouterEngine {
	if limiter == nil {
		limiter = NewRateLimiterManager()
	}
	return &RouterEngine{catalog: catalog, rateLimiter: limiter}
}

func (e *RouterEngine) RouteByName(ctx context.Context, providerName string, modelName string, payload map[string]any) (map[string]any, error) {
	if e.catalog == nil {
		return nil, &RoutingError{Message: "router catalog unavailable"}
	}
	providerName = strings.TrimSpace(providerName)
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil, &RoutingError{Message: "model name is required"}
	}

	if providerName != "" {
		_, err := e.catalog.GetModelByProviderAndName(ctx, providerName, modelName)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, &RoutingError{Message: "model not found"}
			}
			return nil, err
		}
		return e.catalog.OpenAIChatCompletions(ctx, providerName, ensureModelPayload(payload, providerName, modelName))
	}

	models, err := e.catalog.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	candidates := filterActiveModelsByName(models, modelName)
	if len(candidates) == 0 {
		return nil, &RoutingError{Message: "no active model matched"}
	}
	selected := selectHighestPriorityModel(candidates)
	return e.catalog.OpenAIChatCompletions(ctx, selected.ProviderName, ensureModelPayload(payload, selected.ProviderName, selected.Name))
}

func (e *RouterEngine) RouteByTags(ctx context.Context, tags []string, payload map[string]any) (map[string]any, error) {
	if e.catalog == nil {
		return nil, &RoutingError{Message: "router catalog unavailable"}
	}
	models, err := e.catalog.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	candidates := filterModelsByTags(models, tags)
	if len(candidates) == 0 {
		return nil, &RoutingError{Message: "no active model matched tags"}
	}
	selected := selectHighestPriorityModel(candidates)
	payload = ensureModelPayload(payload, selected.ProviderName, selected.Name)
	return e.catalog.OpenAIChatCompletions(ctx, selected.ProviderName, payload)
}

func ensureModelPayload(payload map[string]any, providerName, modelName string) map[string]any {
	out := map[string]any{}
	for k, v := range payload {
		out[k] = v
	}
	if _, ok := out["model"]; !ok {
		out["model"] = modelName
	}
	if providerName != "" {
		out["provider"] = providerName
	}
	return out
}

func filterActiveModelsByName(models []schemas.Model, name string) []schemas.Model {
	out := make([]schemas.Model, 0)
	for _, m := range models {
		if !m.IsActive {
			continue
		}
		if strings.TrimSpace(m.Name) == name {
			out = append(out, m)
		}
	}
	return out
}

func filterModelsByTags(models []schemas.Model, tags []string) []schemas.Model {
	if len(tags) == 0 {
		out := make([]schemas.Model, 0)
		for _, m := range models {
			if m.IsActive {
				out = append(out, m)
			}
		}
		return out
	}
	normalized := make([]string, 0, len(tags))
	for _, t := range tags {
		if v := strings.ToLower(strings.TrimSpace(t)); v != "" {
			normalized = append(normalized, v)
		}
	}
	out := make([]schemas.Model, 0)
	for _, model := range models {
		if !model.IsActive {
			continue
		}
		if hasModelAllTags(model, normalized) {
			out = append(out, model)
		}
	}
	return out
}

func hasModelAllTags(model schemas.Model, tags []string) bool {
	if len(tags) == 0 {
		return true
	}
	modelTags := map[string]struct{}{}
	if model.Config != nil {
		if raw, ok := model.Config["tags"].([]any); ok {
			for _, t := range raw {
				if s, ok := t.(string); ok {
					modelTags[strings.ToLower(strings.TrimSpace(s))] = struct{}{}
				}
			}
		}
		if raw, ok := model.Config["tags"].([]string); ok {
			for _, t := range raw {
				modelTags[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
			}
		}
	}
	for _, want := range tags {
		if _, ok := modelTags[want]; !ok {
			return false
		}
	}
	return true
}

func selectHighestPriorityModel(models []schemas.Model) schemas.Model {
	type candidate struct {
		model    schemas.Model
		priority int64
	}
	items := make([]candidate, 0, len(models))
	for _, model := range models {
		items = append(items, candidate{model: model, priority: modelPriority(model)})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].priority == items[j].priority {
			if items[i].model.ProviderName == items[j].model.ProviderName {
				return items[i].model.Name < items[j].model.Name
			}
			return items[i].model.ProviderName < items[j].model.ProviderName
		}
		return items[i].priority > items[j].priority
	})
	if len(items) == 0 {
		panic(fmt.Errorf("selectHighestPriorityModel requires non-empty models"))
	}
	return items[0].model
}

func modelPriority(model schemas.Model) int64 {
	if model.Config == nil {
		return 0
	}
	if raw, ok := model.Config["priority"]; ok {
		switch v := raw.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}
