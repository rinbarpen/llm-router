package services

import (
	"sort"
	"strings"
)

// CodexModel describes codex-capable model metadata.
type CodexModel struct {
	Name        string   `json:"name"`
	Provider    string   `json:"provider"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// CodexCatalog is an in-memory catalog used by codex-related endpoints.
type CodexCatalog struct {
	models []CodexModel
}

func NewCodexCatalog(models []CodexModel) *CodexCatalog {
	copied := make([]CodexModel, len(models))
	copy(copied, models)
	return &CodexCatalog{models: copied}
}

func (c *CodexCatalog) List() []CodexModel {
	out := make([]CodexModel, len(c.models))
	copy(out, c.models)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Provider == out[j].Provider {
			return out[i].Name < out[j].Name
		}
		return out[i].Provider < out[j].Provider
	})
	return out
}

func (c *CodexCatalog) Search(keyword string) []CodexModel {
	kw := strings.ToLower(strings.TrimSpace(keyword))
	if kw == "" {
		return c.List()
	}
	out := make([]CodexModel, 0)
	for _, m := range c.models {
		if strings.Contains(strings.ToLower(m.Name), kw) || strings.Contains(strings.ToLower(m.Provider), kw) {
			out = append(out, m)
			continue
		}
		for _, t := range m.Tags {
			if strings.Contains(strings.ToLower(t), kw) {
				out = append(out, m)
				break
			}
		}
	}
	return out
}
