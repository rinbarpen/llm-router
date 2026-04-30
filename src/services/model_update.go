package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/rinbarpen/llm-router/src/schemas"
)

const (
	ModelAutoUpdateManager       = "model_auto_update"
	AutoManagedModelsBeginMarker = "# BEGIN AUTO-MANAGED MODELS"
	AutoManagedModelsEndMarker   = "# END AUTO-MANAGED MODELS"
)

type DiscoveredModel struct {
	Name             string         `json:"id"`
	DisplayName      string         `json:"display_name,omitempty"`
	RemoteIdentifier string         `json:"remote_identifier,omitempty"`
	Description      string         `json:"description,omitempty"`
	Config           map[string]any `json:"config,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	ContextWindow    string         `json:"context_window,omitempty"`
	SupportsVision   *bool          `json:"supports_vision,omitempty"`
	SupportsTools    *bool          `json:"supports_tools,omitempty"`
	Languages        []string       `json:"languages,omitempty"`
}

type modelSourceFile struct {
	ProviderName string            `json:"provider_name"`
	ProviderType string            `json:"provider_type"`
	Models       []DiscoveredModel `json:"models"`
}

type MergeModelOptions struct {
	DefaultNewModelActive bool
	ManagedAt             string
}

type MergeModelResult struct {
	Models  []schemas.Model
	Added   []string
	Updated []string
}

type ModelUpdateRun struct {
	ProviderName string    `json:"provider_name"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	Added        []string  `json:"added"`
	Updated      []string  `json:"updated"`
	Deleted      []string  `json:"deleted"`
	Disabled     []string  `json:"disabled,omitempty"`
	Skipped      []string  `json:"skipped"`
	Error        string    `json:"error,omitempty"`
	BackupPath   string    `json:"backup_path,omitempty"`
}

type ModelUpdateResult struct {
	StartedAt    time.Time        `json:"started_at"`
	CompletedAt  time.Time        `json:"completed_at"`
	ProviderRuns []ModelUpdateRun `json:"provider_runs"`
	BackupPath   string           `json:"backup_path,omitempty"`
}

type ModelUpdateStatusStore struct {
	mu    sync.Mutex
	limit int
	runs  []ModelUpdateRun
}

func NewModelUpdateStatusStore(limit int) *ModelUpdateStatusStore {
	if limit <= 0 {
		limit = 20
	}
	return &ModelUpdateStatusStore{limit: limit}
}

func (s *ModelUpdateStatusStore) Record(run ModelUpdateRun) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs = append(s.runs, run)
	if len(s.runs) > s.limit {
		s.runs = s.runs[len(s.runs)-s.limit:]
	}
}

func (s *ModelUpdateStatusStore) Latest() (ModelUpdateRun, bool) {
	if s == nil {
		return ModelUpdateRun{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.runs) == 0 {
		return ModelUpdateRun{}, false
	}
	return s.runs[len(s.runs)-1], true
}

func (s *ModelUpdateStatusStore) Runs() []ModelUpdateRun {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ModelUpdateRun, len(s.runs))
	copy(out, s.runs)
	return out
}

type ModelUpdateDeps struct {
	ListProviders            func(context.Context) ([]schemas.Provider, error)
	ListModelsByProvider     func(context.Context, string) ([]schemas.Model, error)
	FetchProviderModels      func(context.Context, schemas.Provider) ([]DiscoveredModel, error)
	DeleteAutoManagedMissing func(context.Context, string, []string) error
	WriteRouterTOML          func(context.Context, []schemas.Model) (string, error)
	SyncRouterTOML           func(context.Context) error
	StatusStore              *ModelUpdateStatusStore
}

type ModelUpdateOptions struct {
	DefaultNewModelActive bool
	WriteRouterTOML       bool
	ProviderFilters       []string
	DryRun                bool
}

func RunModelUpdate(ctx context.Context, deps ModelUpdateDeps, opts ModelUpdateOptions) (ModelUpdateResult, error) {
	started := time.Now().UTC()
	result := ModelUpdateResult{StartedAt: started}
	if deps.ListProviders == nil {
		return result, fmt.Errorf("list providers dependency is required")
	}
	providers, err := deps.ListProviders(ctx)
	if err != nil {
		return result, err
	}
	if len(providers) == 0 {
		result.CompletedAt = time.Now().UTC()
		if deps.StatusStore != nil {
			deps.StatusStore.Record(ModelUpdateRun{StartedAt: started, CompletedAt: result.CompletedAt})
		}
		return result, nil
	}
	allManaged := make([]schemas.Model, 0)
	for _, provider := range providers {
		selected := providerMatchesFilters(provider, opts.ProviderFilters)
		run := ModelUpdateRun{ProviderName: provider.Name, StartedAt: time.Now().UTC()}
		if !selected {
			if deps.ListModelsByProvider != nil {
				if local, listErr := deps.ListModelsByProvider(ctx, provider.Name); listErr == nil {
					allManaged = append(allManaged, managedModelsOnly(local)...)
				}
			}
			continue
		}
		if deps.FetchProviderModels == nil || deps.ListModelsByProvider == nil {
			run.Error = "model update dependencies are incomplete"
			run.CompletedAt = time.Now().UTC()
			result.ProviderRuns = append(result.ProviderRuns, run)
			if deps.StatusStore != nil {
				deps.StatusStore.Record(run)
			}
			continue
		}
		local, listErr := deps.ListModelsByProvider(ctx, provider.Name)
		if listErr != nil {
			run.Error = listErr.Error()
			run.CompletedAt = time.Now().UTC()
			result.ProviderRuns = append(result.ProviderRuns, run)
			if deps.StatusStore != nil {
				deps.StatusStore.Record(run)
			}
			continue
		}
		discovered, fetchErr := deps.FetchProviderModels(ctx, provider)
		if fetchErr != nil {
			run.Error = fetchErr.Error()
			run.CompletedAt = time.Now().UTC()
			result.ProviderRuns = append(result.ProviderRuns, run)
			if deps.StatusStore != nil {
				deps.StatusStore.Record(run)
			}
			continue
		}
		if len(discovered) == 0 {
			run.Skipped = []string{"no models discovered"}
			allManaged = append(allManaged, managedModelsOnly(local)...)
			run.CompletedAt = time.Now().UTC()
			result.ProviderRuns = append(result.ProviderRuns, run)
			if deps.StatusStore != nil {
				deps.StatusStore.Record(run)
			}
			continue
		}
		merged := MergeDiscoveredModels(provider.Name, local, discovered, MergeModelOptions{
			DefaultNewModelActive: opts.DefaultNewModelActive,
			ManagedAt:             time.Now().UTC().Format(time.RFC3339),
		})
		run.Added = merged.Added
		run.Updated = merged.Updated
		run.Deleted = autoManagedMissingNames(local, discovered)
		if len(run.Deleted) > 0 && deps.DeleteAutoManagedMissing != nil && !opts.DryRun {
			if err := deps.DeleteAutoManagedMissing(ctx, provider.Name, run.Deleted); err != nil {
				run.Error = err.Error()
			}
		}
		allManaged = append(allManaged, managedModelsOnly(merged.Models)...)
		run.CompletedAt = time.Now().UTC()
		result.ProviderRuns = append(result.ProviderRuns, run)
		if deps.StatusStore != nil {
			deps.StatusStore.Record(run)
		}
	}
	if opts.WriteRouterTOML && deps.WriteRouterTOML != nil && !opts.DryRun {
		backup, writeErr := deps.WriteRouterTOML(ctx, allManaged)
		result.BackupPath = backup
		for i := range result.ProviderRuns {
			result.ProviderRuns[i].BackupPath = backup
		}
		if writeErr != nil {
			return result, writeErr
		}
		if deps.SyncRouterTOML != nil {
			if err := deps.SyncRouterTOML(ctx); err != nil {
				return result, err
			}
		}
	}
	result.CompletedAt = time.Now().UTC()
	return result, nil
}

func (s *CatalogService) RunModelUpdate(ctx context.Context, configPath string, sourceDir string, writeRouterTOML bool, defaultNewModelActive bool, providerFilters ...string) (ModelUpdateResult, error) {
	return s.RunModelUpdateWithOptions(ctx, configPath, sourceDir, ModelUpdateOptions{
		WriteRouterTOML:       writeRouterTOML,
		DefaultNewModelActive: defaultNewModelActive,
		ProviderFilters:       providerFilters,
	})
}

func (s *CatalogService) RunModelUpdateWithOptions(ctx context.Context, configPath string, sourceDir string, opts ModelUpdateOptions) (ModelUpdateResult, error) {
	if strings.TrimSpace(sourceDir) == "" {
		sourceDir = "data/model_sources"
	}
	deps := ModelUpdateDeps{
		ListProviders:        s.ListProviders,
		ListModelsByProvider: s.ListModelsByProvider,
		FetchProviderModels: func(ctx context.Context, provider schemas.Provider) ([]DiscoveredModel, error) {
			return s.discoverProviderModels(ctx, provider, sourceDir)
		},
		DeleteAutoManagedMissing: s.DeleteAutoManagedModels,
		WriteRouterTOML: func(ctx context.Context, models []schemas.Model) (string, error) {
			return WriteAutoManagedModelsToRouterTOML(configPath, models)
		},
		SyncRouterTOML: func(ctx context.Context) error {
			return s.SyncRouterTOML(ctx, configPath)
		},
		StatusStore: s.modelUpdateStatus,
	}
	return RunModelUpdate(ctx, deps, opts)
}

func (s *CatalogService) LatestModelUpdateRun(ctx context.Context) (ModelUpdateRun, bool, error) {
	_ = ctx
	if s.modelUpdateStatus == nil {
		return ModelUpdateRun{}, false, nil
	}
	run, ok := s.modelUpdateStatus.Latest()
	return run, ok, nil
}

func (s *CatalogService) ListModelUpdateRuns(ctx context.Context) ([]ModelUpdateRun, error) {
	_ = ctx
	if s.modelUpdateStatus == nil {
		return nil, nil
	}
	return s.modelUpdateStatus.Runs(), nil
}

func (s *CatalogService) discoverProviderModels(ctx context.Context, provider schemas.Provider, sourceDir string) ([]DiscoveredModel, error) {
	rows, err := s.fetchProviderModels(ctx, provider)
	if err == nil && len(rows) > 0 {
		return discoveredFromCatalogRows(provider, rows), nil
	}
	sourceRows, sourceErr := LoadModelSourceFiles(sourceDir, provider)
	if sourceErr != nil {
		return nil, sourceErr
	}
	if len(sourceRows) > 0 {
		return sourceRows, nil
	}
	if errors.Is(err, ErrNotImplemented) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func providerMatchesFilters(provider schemas.Provider, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(provider.Name))
	pt := strings.ToLower(strings.TrimSpace(provider.Type))
	for _, filter := range filters {
		filter = strings.ToLower(strings.TrimSpace(filter))
		if filter == "" {
			continue
		}
		if filter == name || filter == pt {
			return true
		}
	}
	return false
}

func discoveredFromCatalogRows(provider schemas.Provider, rows []map[string]any) []DiscoveredModel {
	out := make([]DiscoveredModel, 0, len(rows))
	for _, row := range rows {
		remoteID, _ := row["model_name"].(string)
		if strings.TrimSpace(remoteID) == "" {
			continue
		}
		metadata, _ := row["metadata"].(map[string]any)
		name := remoteID
		pt := strings.ToLower(strings.TrimSpace(provider.Type))
		if pt == "openrouter" || strings.Contains(remoteID, "/") {
			name = normalizeLocalModelName(remoteID)
		}
		displayName := ""
		if metadata != nil {
			if v, ok := metadata["name"].(string); ok {
				displayName = v
			}
		}
		cfg := map[string]any{}
		if metadata != nil {
			if v, ok := metadata["context_length"]; ok {
				cfg["context_window"] = humanContextWindow(v)
			}
			if arch, ok := metadata["architecture"].(map[string]any); ok {
				if v, ok := arch["input_modalities"]; ok {
					cfg["supports_vision"] = modalitiesInclude(v, "image")
				}
				if v, ok := arch["modality"].(string); ok && strings.Contains(strings.ToLower(v), "image") {
					cfg["supports_vision"] = true
				}
				if v, ok := arch["function_calling"].(bool); ok {
					cfg["supports_tools"] = v
				}
			}
		}
		tags := inferModelTags(provider, remoteID, cfg)
		out = append(out, normalizeDiscoveredModel(DiscoveredModel{
			Name:             name,
			DisplayName:      displayName,
			RemoteIdentifier: remoteID,
			Config:           cfg,
			Tags:             tags,
		}))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func WriteAutoManagedModelsToRouterTOML(configPath string, models []schemas.Model) (string, error) {
	if strings.TrimSpace(configPath) == "" {
		configPath = "router.toml"
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	next, err := ReplaceAutoManagedModelBlock(string(raw), models)
	if err != nil {
		return "", err
	}
	backupPath := fmt.Sprintf("%s.model-auto-update.%s.bak", configPath, time.Now().UTC().Format("20060102T150405Z"))
	if err := os.WriteFile(backupPath, raw, 0o644); err != nil {
		return "", err
	}
	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(next), 0o644); err != nil {
		return backupPath, err
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		return backupPath, err
	}
	return backupPath, nil
}

func (s *CatalogService) DeleteAutoManagedModels(ctx context.Context, providerName string, names []string) error {
	if len(names) == 0 {
		return nil
	}
	args := make([]any, 0, len(names)+2)
	args = append(args, providerName)
	for _, name := range names {
		args = append(args, name)
	}
	args = append(args, ModelAutoUpdateManager)
	_, err := s.pool.Exec(ctx, `
		DELETE FROM models
		WHERE provider_id = (SELECT id FROM providers WHERE name = $1)
		  AND name IN (`+makePlaceholders(len(names))+`)
		  AND json_extract(config, '$.managed_by') = $`+fmt.Sprintf("%d", len(args))+`
	`, args...)
	if err != nil {
		return fmt.Errorf("delete auto-managed models: %w", err)
	}
	return nil
}

func LoadModelSourceFiles(sourceDir string, provider schemas.Provider) ([]DiscoveredModel, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []DiscoveredModel
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(sourceDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var source modelSourceFile
		if err := json.Unmarshal(raw, &source); err != nil {
			return nil, fmt.Errorf("parse model source %s: %w", entry.Name(), err)
		}
		if !sourceMatchesProvider(source, provider) {
			continue
		}
		for _, model := range source.Models {
			out = append(out, normalizeDiscoveredModel(model))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func sourceMatchesProvider(source modelSourceFile, provider schemas.Provider) bool {
	if source.ProviderName != "" && strings.EqualFold(source.ProviderName, provider.Name) {
		return true
	}
	return source.ProviderType != "" && strings.EqualFold(source.ProviderType, provider.Type)
}

func MergeDiscoveredModels(providerName string, existing []schemas.Model, discovered []DiscoveredModel, opts MergeModelOptions) MergeModelResult {
	byName := make(map[string]schemas.Model, len(existing))
	for _, model := range existing {
		byName[model.Name] = model
	}
	out := make([]schemas.Model, 0, len(discovered))
	result := MergeModelResult{}
	for _, item := range discovered {
		item = normalizeDiscoveredModel(item)
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		if current, ok := byName[item.Name]; ok {
			current.ProviderName = providerName
			current.Config = mergeAutoMetadata(current.Config, item, opts.ManagedAt)
			if opts.DefaultNewModelActive && isAutoManagedModel(current) {
				current.IsActive = true
			}
			out = append(out, current)
			result.Updated = append(result.Updated, current.Name)
			continue
		}
		displayName := item.DisplayName
		if strings.TrimSpace(displayName) == "" {
			displayName = item.Name
		}
		remoteID := item.RemoteIdentifier
		if strings.TrimSpace(remoteID) == "" {
			remoteID = item.Name
		}
		model := schemas.Model{
			ProviderName:     providerName,
			Name:             item.Name,
			DisplayName:      &displayName,
			Description:      stringPtrIfNotEmpty(item.Description),
			IsActive:         opts.DefaultNewModelActive,
			RemoteIdentifier: &remoteID,
			DefaultParams:    map[string]any{},
			Config:           mergeAutoMetadata(item.Config, item, opts.ManagedAt),
			CreatedAt:        nil,
			UpdatedAt:        nil,
		}
		out = append(out, model)
		result.Added = append(result.Added, model.Name)
	}
	result.Models = out
	sort.Strings(result.Added)
	sort.Strings(result.Updated)
	return result
}

func ReplaceAutoManagedModelBlock(input string, models []schemas.Model) (string, error) {
	body, err := renderAutoManagedModelBlock(models)
	if err != nil {
		return "", err
	}
	without := removeAutoManagedModelBlock(input)
	without = strings.TrimRight(without, "\n")
	if body == "" {
		return without + "\n", nil
	}
	return without + "\n\n" + body, nil
}

func renderAutoManagedModelBlock(models []schemas.Model) (string, error) {
	if len(models) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(AutoManagedModelsBeginMarker)
	b.WriteString("\n")
	sort.Slice(models, func(i, j int) bool {
		if models[i].ProviderName == models[j].ProviderName {
			return models[i].Name < models[j].Name
		}
		return models[i].ProviderName < models[j].ProviderName
	})
	for _, model := range models {
		cfg := model.Config
		if cfg == nil {
			cfg = map[string]any{}
		}
		b.WriteString("\n[[models]]\n")
		writeTOMLString(&b, "name", model.Name)
		writeTOMLString(&b, "provider", model.ProviderName)
		b.WriteString(fmt.Sprintf("is_active = %v\n", model.IsActive))
		if model.RemoteIdentifier != nil && strings.TrimSpace(*model.RemoteIdentifier) != "" {
			writeTOMLString(&b, "remote_identifier", *model.RemoteIdentifier)
		}
		if model.DisplayName != nil && strings.TrimSpace(*model.DisplayName) != "" {
			writeTOMLString(&b, "display_name", *model.DisplayName)
		}
		if tags := configStringSlice(cfg["tags"]); len(tags) > 0 {
			writeTOMLStringArray(&b, "tags", tags)
		}
		cleanCfg := make(map[string]any, len(cfg))
		for k, v := range cfg {
			if k == "tags" {
				continue
			}
			cleanCfg[k] = v
		}
		if len(cleanCfg) > 0 {
			raw, err := toml.Marshal(cleanCfg)
			if err != nil {
				return "", err
			}
			b.WriteString("[models.config]\n")
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
				if strings.TrimSpace(line) != "" {
					b.WriteString(line)
					b.WriteString("\n")
				}
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(AutoManagedModelsEndMarker)
	b.WriteString("\n")
	return b.String(), nil
}

func removeAutoManagedModelBlock(input string) string {
	start := strings.Index(input, AutoManagedModelsBeginMarker)
	if start < 0 {
		return input
	}
	end := strings.Index(input[start:], AutoManagedModelsEndMarker)
	if end < 0 {
		return input[:start]
	}
	end += start + len(AutoManagedModelsEndMarker)
	for end < len(input) && (input[end] == '\n' || input[end] == '\r') {
		end++
	}
	return input[:start] + input[end:]
}

func normalizeDiscoveredModel(model DiscoveredModel) DiscoveredModel {
	model.Name = strings.TrimSpace(model.Name)
	if model.RemoteIdentifier == "" {
		model.RemoteIdentifier = model.Name
	}
	if model.Config == nil {
		model.Config = map[string]any{}
	}
	if model.ContextWindow != "" {
		model.Config["context_window"] = model.ContextWindow
	}
	if model.SupportsVision != nil {
		model.Config["supports_vision"] = *model.SupportsVision
	}
	if model.SupportsTools != nil {
		model.Config["supports_tools"] = *model.SupportsTools
	}
	if len(model.Languages) > 0 {
		items := make([]any, len(model.Languages))
		for i, lang := range model.Languages {
			items[i] = lang
		}
		model.Config["languages"] = items
	}
	if len(model.Tags) > 0 {
		items := make([]any, len(model.Tags))
		for i, tag := range model.Tags {
			items[i] = tag
		}
		model.Config["tags"] = items
	}
	return model
}

func mergeAutoMetadata(cfg map[string]any, item DiscoveredModel, managedAt string) map[string]any {
	out := cloneAnyMap(cfg)
	if out == nil {
		out = map[string]any{}
	}
	for key, value := range normalizeDiscoveredModel(item).Config {
		if _, exists := out[key]; !exists {
			out[key] = value
		}
	}
	out["managed_by"] = ModelAutoUpdateManager
	out["source_model_id"] = item.RemoteIdentifier
	out["last_seen_at"] = managedAt
	return out
}

func managedModelsOnly(models []schemas.Model) []schemas.Model {
	out := make([]schemas.Model, 0, len(models))
	for _, model := range models {
		if model.Config != nil && model.Config["managed_by"] == ModelAutoUpdateManager {
			out = append(out, model)
		}
	}
	return out
}

func autoManagedMissingNames(existing []schemas.Model, discovered []DiscoveredModel) []string {
	seen := make(map[string]struct{}, len(discovered))
	for _, item := range discovered {
		item = normalizeDiscoveredModel(item)
		seen[item.Name] = struct{}{}
	}
	var missing []string
	for _, model := range existing {
		if model.Config == nil || model.Config["managed_by"] != ModelAutoUpdateManager {
			continue
		}
		if _, ok := seen[model.Name]; !ok {
			missing = append(missing, model.Name)
		}
	}
	sort.Strings(missing)
	return missing
}

func writeTOMLString(b *strings.Builder, key, value string) {
	raw, _ := json.Marshal(value)
	b.WriteString(key)
	b.WriteString(" = ")
	b.Write(raw)
	b.WriteString("\n")
}

func writeTOMLStringArray(b *strings.Builder, key string, values []string) {
	raw, _ := json.Marshal(values)
	b.WriteString(key)
	b.WriteString(" = ")
	b.Write(raw)
	b.WriteString("\n")
}

func configStringSlice(v any) []string {
	switch items := v.(type) {
	case []string:
		return items
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func stringPtrIfNotEmpty(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return &v
}

func normalizeLocalModelName(remoteID string) string {
	name := strings.ToLower(strings.TrimSpace(remoteID))
	name = strings.TrimSuffix(name, ":free")
	name = strings.ReplaceAll(name, "/", "-")
	name = regexp.MustCompile(`[^a-z0-9._-]+`).ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return remoteID
	}
	return name
}

func humanContextWindow(v any) string {
	var n int64
	switch x := v.(type) {
	case int64:
		n = x
	case int:
		n = int64(x)
	case float64:
		n = int64(x)
	case json.Number:
		n, _ = x.Int64()
	}
	if n <= 0 {
		return ""
	}
	if n >= 1000000 && n%1000000 == 0 {
		return fmt.Sprintf("%dM", n/1000000)
	}
	if n >= 1024 {
		return fmt.Sprintf("%dk", (n+999)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func modalitiesInclude(v any, needle string) bool {
	items, ok := v.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if s, ok := item.(string); ok && strings.EqualFold(s, needle) {
			return true
		}
	}
	return false
}

func inferModelTags(provider schemas.Provider, remoteID string, cfg map[string]any) []string {
	tags := []string{"chat", "general", strings.ToLower(strings.TrimSpace(provider.Type))}
	id := strings.ToLower(remoteID)
	if strings.Contains(id, ":free") {
		tags = append(tags, "free")
	}
	if strings.Contains(id, "code") || strings.Contains(id, "coder") {
		tags = append(tags, "code")
	}
	if strings.Contains(id, "reason") || strings.Contains(id, "think") {
		tags = append(tags, "reasoning")
	}
	if cfg["supports_vision"] == true {
		tags = append(tags, "image")
	}
	if cfg["supports_tools"] == true {
		tags = append(tags, "function-call")
	}
	return dedupeStrings(tags)
}

func dedupeStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
