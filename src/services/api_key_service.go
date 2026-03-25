package services

import (
	"strings"

	"github.com/rinbarpen/llm-router/src/schemas"
)

// APIKeyPolicy is a runtime authorization helper derived from DB api key rows.
type APIKeyPolicy struct {
	Key              string
	Name             string
	IsActive         bool
	AllowedModels    []string
	AllowedProviders []string
	ParameterLimits  map[string]any
}

func NewAPIKeyPolicy(item schemas.APIKey) APIKeyPolicy {
	policy := APIKeyPolicy{
		IsActive:         item.IsActive,
		AllowedModels:    normalizeList(item.AllowedModels),
		AllowedProviders: normalizeList(item.AllowedProviders),
		ParameterLimits:  item.ParameterLimits,
	}
	if item.Key != nil {
		policy.Key = strings.TrimSpace(*item.Key)
	}
	if item.Name != nil {
		policy.Name = strings.TrimSpace(*item.Name)
	}
	return policy
}

func (p APIKeyPolicy) IsModelAllowed(providerName string, modelName string) bool {
	if !p.IsActive {
		return false
	}
	providerName = strings.TrimSpace(providerName)
	modelName = strings.TrimSpace(modelName)
	if len(p.AllowedProviders) > 0 && !containsIgnoreCase(p.AllowedProviders, providerName) {
		return false
	}
	if len(p.AllowedModels) == 0 {
		return true
	}
	if containsIgnoreCase(p.AllowedModels, modelName) {
		return true
	}
	compound := providerName + "/" + modelName
	return containsIgnoreCase(p.AllowedModels, compound)
}

func normalizeList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if v := strings.TrimSpace(item); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func containsIgnoreCase(items []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, item := range items {
		if strings.ToLower(strings.TrimSpace(item)) == target {
			return true
		}
	}
	return false
}
