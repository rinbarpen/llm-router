package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/rinbarpen/llm-router/src/config"
)

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			*m = append(*m, item)
		}
	}
	return nil
}

type sourceFile struct {
	ProviderName string        `json:"provider_name"`
	ProviderType string        `json:"provider_type"`
	Models       []sourceModel `json:"models"`
}

type sourceModel struct {
	ID               string         `json:"id"`
	DisplayName      string         `json:"display_name,omitempty"`
	RemoteIdentifier string         `json:"remote_identifier,omitempty"`
	Description      string         `json:"description,omitempty"`
	ContextWindow    string         `json:"context_window,omitempty"`
	SupportsVision   *bool          `json:"supports_vision,omitempty"`
	SupportsTools    *bool          `json:"supports_tools,omitempty"`
	Languages        []string       `json:"languages,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	Config           map[string]any `json:"config,omitempty"`
}

type outputProvider struct {
	Name                 string        `json:"name"`
	Type                 string        `json:"type"`
	IsActive             bool          `json:"is_active"`
	ConfiguredModelCount int           `json:"configured_model_count"`
	SourceModelCount     int           `json:"source_model_count"`
	CombinedModelCount   int           `json:"combined_model_count"`
	Models               []outputModel `json:"models,omitempty"`
}

type outputModel struct {
	Name             string   `json:"name"`
	DisplayName      string   `json:"display_name,omitempty"`
	RemoteIdentifier string   `json:"remote_identifier,omitempty"`
	IsActive         bool     `json:"is_active"`
	Sources          []string `json:"sources"`
	Tags             []string `json:"tags,omitempty"`
	ContextWindow    string   `json:"context_window,omitempty"`
	SupportsVision   *bool    `json:"supports_vision,omitempty"`
	SupportsTools    *bool    `json:"supports_tools,omitempty"`
}

func main() {
	var providers multiFlag
	var (
		configPath      = flag.String("config", "router.toml", "Path to router.toml")
		sourceDir       = flag.String("source-dir", "", "Directory containing data/model_sources/*.json")
		jsonOutput      = flag.Bool("json", false, "Print JSON")
		listProviders   = flag.Bool("providers", false, "List providers only")
		includeInactive = flag.Bool("include-inactive", false, "Include inactive providers and models")
		configuredOnly  = flag.Bool("configured-only", false, "Deprecated: router.toml no longer owns the model catalog")
		sourceOnly      = flag.Bool("source-only", false, "Show models from model source files only")
		namesOnly       = flag.Bool("names-only", false, "Print model names only")
		limit           = flag.Int("limit", 0, "Maximum models to print per provider in text output; 0 means no limit")
	)
	flag.Var(&providers, "provider", "Filter by provider name or type; repeat or comma-separate")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `List supported LLM Router models by provider.

Usage:
  %s [options]

Examples:
  %s
  %s --provider qwen
  %s --provider "qwen (cn)" --source-only
  %s --providers
  %s --json --provider openrouter

Options:
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *configuredOnly {
		fatalf("--configured-only has been removed because router.toml no longer owns the model catalog; query the database/API or use --source-only for static sources")
	}

	cfg, err := config.LoadRouterModelConfigFromTOML(*configPath)
	if err != nil {
		fatalf("%v", err)
	}
	if strings.TrimSpace(*sourceDir) == "" {
		*sourceDir = "data/model_sources"
		if cfg.ModelUpdates != nil && strings.TrimSpace(cfg.ModelUpdates.SourceDir) != "" {
			*sourceDir = cfg.ModelUpdates.SourceDir
		}
	}

	sourceModels, err := loadSourceModels(*sourceDir)
	if err != nil {
		fatalf("%v", err)
	}

	out := buildOutput(cfg, sourceModels, providers, *includeInactive, *configuredOnly, *sourceOnly)

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fatalf("encode json: %v", err)
		}
		return
	}
	if *namesOnly {
		printNames(out, *limit)
		return
	}
	if *listProviders {
		printProviders(out)
		return
	}
	printText(out, *limit)
}

func loadSourceModels(sourceDir string) (map[string][]sourceModel, error) {
	out := map[string][]sourceModel{}
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("read source dir %s: %w", sourceDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(sourceDir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var file sourceFile
		if err := json.Unmarshal(raw, &file); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		key := strings.TrimSpace(file.ProviderName)
		if key == "" {
			key = strings.TrimSpace(file.ProviderType)
		}
		if key == "" {
			continue
		}
		out[normalizeKey(key)] = append(out[normalizeKey(key)], file.Models...)
	}
	for key := range out {
		sort.SliceStable(out[key], func(i, j int) bool {
			return strings.ToLower(out[key][i].ID) < strings.ToLower(out[key][j].ID)
		})
	}
	return out, nil
}

func buildOutput(cfg config.RouterModelConfig, sourceRows map[string][]sourceModel, filters []string, includeInactive, configuredOnly, sourceOnly bool) []outputProvider {
	modelsByProvider := map[string][]config.ModelConfigEntry{}
	for _, model := range cfg.Models {
		if !includeInactive && !model.IsActive {
			continue
		}
		key := normalizeKey(model.Provider)
		modelsByProvider[key] = append(modelsByProvider[key], model)
	}
	for key := range modelsByProvider {
		sort.SliceStable(modelsByProvider[key], func(i, j int) bool {
			return strings.ToLower(modelsByProvider[key][i].Name) < strings.ToLower(modelsByProvider[key][j].Name)
		})
	}

	filterSet := map[string]struct{}{}
	for _, item := range filters {
		filterSet[normalizeKey(item)] = struct{}{}
	}

	out := make([]outputProvider, 0, len(cfg.Providers))
	for _, provider := range cfg.Providers {
		if !includeInactive && !provider.IsActive {
			continue
		}
		nameKey := normalizeKey(provider.Name)
		typeKey := normalizeKey(provider.Type)
		if len(filterSet) > 0 {
			if _, ok := filterSet[nameKey]; !ok {
				if _, ok := filterSet[typeKey]; !ok {
					continue
				}
			}
		}

		combined := map[string]*outputModel{}
		if !sourceOnly {
			for _, model := range modelsByProvider[nameKey] {
				row := ensureModel(combined, model.Name)
				row.DisplayName = stringValue(model.DisplayName, row.DisplayName)
				row.RemoteIdentifier = stringValue(model.RemoteIdentifier, row.RemoteIdentifier)
				row.IsActive = model.IsActive
				row.Tags = mergeStrings(row.Tags, model.Tags)
				row.ContextWindow = firstNonEmpty(row.ContextWindow, configString(model.Config, "context_window"))
				row.SupportsVision = firstBool(row.SupportsVision, configBool(model.Config, "supports_vision"))
				row.SupportsTools = firstBool(row.SupportsTools, configBool(model.Config, "supports_tools"))
				row.Sources = appendSource(row.Sources, "configured")
			}
		}

		if !configuredOnly {
			addSourceRows(combined, sourceRows[nameKey])
			if nameKey != typeKey {
				addSourceRows(combined, sourceRows[typeKey])
			}
		}

		models := sortedModels(combined)
		out = append(out, outputProvider{
			Name:                 provider.Name,
			Type:                 provider.Type,
			IsActive:             provider.IsActive,
			ConfiguredModelCount: countBySource(models, "configured"),
			SourceModelCount:     countBySource(models, "source"),
			CombinedModelCount:   len(models),
			Models:               models,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func addSourceRows(combined map[string]*outputModel, rows []sourceModel) {
	for _, model := range rows {
		if strings.TrimSpace(model.ID) == "" {
			continue
		}
		row := ensureModel(combined, model.ID)
		row.DisplayName = firstNonEmpty(row.DisplayName, model.DisplayName)
		row.RemoteIdentifier = firstNonEmpty(row.RemoteIdentifier, model.RemoteIdentifier)
		row.Tags = mergeStrings(row.Tags, model.Tags)
		row.ContextWindow = firstNonEmpty(row.ContextWindow, model.ContextWindow)
		row.SupportsVision = firstBool(row.SupportsVision, model.SupportsVision)
		row.SupportsTools = firstBool(row.SupportsTools, model.SupportsTools)
		row.Sources = appendSource(row.Sources, "source")
	}
}

func ensureModel(rows map[string]*outputModel, name string) *outputModel {
	key := normalizeKey(name)
	if row, ok := rows[key]; ok {
		return row
	}
	row := &outputModel{Name: strings.TrimSpace(name), IsActive: true}
	rows[key] = row
	return row
}

func sortedModels(rows map[string]*outputModel) []outputModel {
	out := make([]outputModel, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func countBySource(rows []outputModel, source string) int {
	count := 0
	for _, row := range rows {
		for _, item := range row.Sources {
			if item == source {
				count++
				break
			}
		}
	}
	return count
}

func printProviders(rows []outputProvider) {
	fmt.Printf("%-28s %-18s %8s %10s %8s\n", "PROVIDER", "TYPE", "ACTIVE", "CONFIGURED", "SOURCE")
	for _, row := range rows {
		fmt.Printf("%-28s %-18s %8s %10d %8d\n", row.Name, row.Type, strconv.FormatBool(row.IsActive), row.ConfiguredModelCount, row.SourceModelCount)
	}
}

func printNames(rows []outputProvider, limit int) {
	for _, provider := range rows {
		models := limitedModels(provider.Models, limit)
		for _, model := range models {
			fmt.Printf("%s/%s\n", provider.Name, model.Name)
		}
	}
}

func printText(rows []outputProvider, limit int) {
	if len(rows) == 0 {
		fmt.Println("No providers matched.")
		return
	}
	for i, provider := range rows {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("=== %s (type=%s, active=%s) ===\n", provider.Name, provider.Type, strconv.FormatBool(provider.IsActive))
		fmt.Printf("configured=%d source=%d combined=%d\n", provider.ConfiguredModelCount, provider.SourceModelCount, provider.CombinedModelCount)
		models := limitedModels(provider.Models, limit)
		for _, model := range models {
			parts := []string{model.Name}
			if model.DisplayName != "" && model.DisplayName != model.Name {
				parts = append(parts, "- "+model.DisplayName)
			}
			if len(model.Sources) > 0 {
				parts = append(parts, "["+strings.Join(model.Sources, ",")+"]")
			}
			if model.ContextWindow != "" {
				parts = append(parts, "ctx="+model.ContextWindow)
			}
			fmt.Printf("  - %s\n", strings.Join(parts, " "))
		}
		if limit > 0 && len(provider.Models) > limit {
			fmt.Printf("  ... %d more\n", len(provider.Models)-limit)
		}
	}
}

func limitedModels(rows []outputModel, limit int) []outputModel {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func normalizeKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func stringValue(ptr *string, fallback string) string {
	if ptr == nil || strings.TrimSpace(*ptr) == "" {
		return fallback
	}
	return strings.TrimSpace(*ptr)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func configString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	switch value := cfg[key].(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		return ""
	}
}

func configBool(cfg map[string]any, key string) *bool {
	if cfg == nil {
		return nil
	}
	if value, ok := cfg[key].(bool); ok {
		return &value
	}
	return nil
}

func firstBool(values ...*bool) *bool {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func mergeStrings(left, right []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(left)+len(right))
	for _, value := range append(left, right...) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := normalizeKey(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func appendSource(sources []string, source string) []string {
	for _, item := range sources {
		if item == source {
			return sources
		}
	}
	return append(sources, source)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
