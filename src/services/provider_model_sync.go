package services

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/rinbarpen/llm-router/src/schemas"
)

type RemoteProviderModel struct {
	ProviderName     string         `json:"provider_name"`
	ProviderType     string         `json:"provider_type"`
	ModelName        string         `json:"model_name"`
	LocalName        string         `json:"local_name"`
	DisplayName      string         `json:"display_name,omitempty"`
	RemoteIdentifier string         `json:"remote_identifier"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type ProviderModelSyncOptions struct {
	DefaultNewModelActive bool
	ProviderFilters       []string
}

func (s *CatalogService) ListProviderRemoteModels(ctx context.Context, providerName string, refresh bool) ([]RemoteProviderModel, error) {
	provider, err := s.GetProviderByName(ctx, providerName)
	if err != nil {
		return nil, err
	}
	rows, err := s.fetchProviderModelsLive(ctx, provider, refresh)
	if err != nil {
		return nil, err
	}
	return remoteProviderModelsFromRows(provider, rows), nil
}

func (s *CatalogService) SyncProviderModelsFromRemote(ctx context.Context, providerName string, defaultNewModelActive bool) (ModelUpdateRun, error) {
	provider, err := s.GetProviderByName(ctx, providerName)
	if err != nil {
		return ModelUpdateRun{}, err
	}
	return s.syncProviderModelsFromRemote(ctx, provider, defaultNewModelActive)
}

func (s *CatalogService) SyncAllProviderModelsFromRemote(ctx context.Context, opts ProviderModelSyncOptions) (ModelUpdateResult, error) {
	started := time.Now().UTC()
	result := ModelUpdateResult{StartedAt: started}
	providers, err := s.ListProviders(ctx)
	if err != nil {
		return result, err
	}
	for _, provider := range providers {
		if !providerMatchesFilters(provider, opts.ProviderFilters) {
			continue
		}
		run, _ := s.syncProviderModelsFromRemote(ctx, provider, opts.DefaultNewModelActive)
		result.ProviderRuns = append(result.ProviderRuns, run)
	}
	result.CompletedAt = time.Now().UTC()
	return result, nil
}

func (s *CatalogService) syncProviderModelsFromRemote(ctx context.Context, provider schemas.Provider, defaultNewModelActive bool) (ModelUpdateRun, error) {
	run := ModelUpdateRun{ProviderName: provider.Name, StartedAt: time.Now().UTC()}
	rows, err := s.fetchProviderModelsLive(ctx, provider, true)
	if err != nil {
		run.Error = err.Error()
		run.CompletedAt = time.Now().UTC()
		if s.modelUpdateStatus != nil {
			s.modelUpdateStatus.Record(run)
		}
		return run, nil
	}
	discovered := discoveredFromCatalogRows(provider, rows)
	if len(discovered) == 0 {
		run.Skipped = []string{"no remote models discovered"}
		run.CompletedAt = time.Now().UTC()
		if s.modelUpdateStatus != nil {
			s.modelUpdateStatus.Record(run)
		}
		return run, nil
	}
	local, err := s.ListModelsByProvider(ctx, provider.Name)
	if err != nil {
		run.Error = err.Error()
		run.CompletedAt = time.Now().UTC()
		if s.modelUpdateStatus != nil {
			s.modelUpdateStatus.Record(run)
		}
		return run, nil
	}

	localByName := make(map[string]schemas.Model, len(local))
	localByRemoteID := make(map[string]schemas.Model, len(local))
	for _, model := range local {
		localByName[model.Name] = model
		if model.RemoteIdentifier != nil && strings.TrimSpace(*model.RemoteIdentifier) != "" {
			localByRemoteID[strings.TrimSpace(*model.RemoteIdentifier)] = model
		}
	}

	seenNames := make(map[string]struct{}, len(discovered))
	seenRemoteIDs := make(map[string]struct{}, len(discovered))
	now := time.Now().UTC().Format(time.RFC3339)
	for _, item := range discovered {
		item = normalizeDiscoveredModel(item)
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		seenNames[item.Name] = struct{}{}
		seenRemoteIDs[item.RemoteIdentifier] = struct{}{}

		current, ok := localByRemoteID[item.RemoteIdentifier]
		if !ok {
			current, ok = localByName[item.Name]
		}
		if ok {
			update := syncUpdateForDiscoveredModel(current, item, now)
			if defaultNewModelActive && isAutoManagedModel(current) && !current.IsActive {
				active := true
				update.IsActive = &active
			} else if isAutoManagedModel(current) && !current.IsActive && current.Config != nil && current.Config["last_missing_at"] != nil {
				active := true
				update.IsActive = &active
			}
			if _, err := s.UpdateModel(ctx, provider.Name, current.Name, update); err != nil {
				run.Error = err.Error()
				continue
			}
			run.Updated = append(run.Updated, current.Name)
			continue
		}

		displayName := item.DisplayName
		if strings.TrimSpace(displayName) == "" {
			displayName = item.Name
		}
		remoteID := item.RemoteIdentifier
		model, err := s.CreateModel(ctx, schemas.ModelCreate{
			ProviderName:     provider.Name,
			Name:             item.Name,
			DisplayName:      &displayName,
			Description:      stringPtrIfNotEmpty(item.Description),
			RemoteIdentifier: &remoteID,
			DefaultParams:    map[string]any{},
			Config:           mergeAutoMetadata(item.Config, item, now),
		})
		if err != nil {
			run.Error = err.Error()
			continue
		}
		if !defaultNewModelActive {
			inactive := false
			if _, err := s.UpdateModel(ctx, provider.Name, model.Name, schemas.ModelUpdate{IsActive: &inactive}); err != nil {
				run.Error = err.Error()
				continue
			}
		}
		run.Added = append(run.Added, item.Name)
	}

	for _, model := range local {
		if !isAutoManagedModel(model) {
			continue
		}
		remoteID := ""
		if model.RemoteIdentifier != nil {
			remoteID = strings.TrimSpace(*model.RemoteIdentifier)
		}
		_, nameSeen := seenNames[model.Name]
		_, remoteSeen := seenRemoteIDs[remoteID]
		if nameSeen || (remoteID != "" && remoteSeen) {
			continue
		}
		cfg := cloneAnyMap(model.Config)
		if cfg == nil {
			cfg = map[string]any{}
		}
		cfg["last_missing_at"] = now
		inactive := false
		if _, err := s.UpdateModel(ctx, provider.Name, model.Name, schemas.ModelUpdate{
			IsActive: &inactive,
			Config:   cfg,
		}); err != nil {
			run.Error = err.Error()
			continue
		}
		run.Disabled = append(run.Disabled, model.Name)
	}

	sort.Strings(run.Added)
	sort.Strings(run.Updated)
	sort.Strings(run.Disabled)
	run.CompletedAt = time.Now().UTC()
	if s.modelUpdateStatus != nil {
		s.modelUpdateStatus.Record(run)
	}
	return run, nil
}

func remoteProviderModelsFromRows(provider schemas.Provider, rows []map[string]any) []RemoteProviderModel {
	discovered := discoveredFromCatalogRows(provider, rows)
	metadataByID := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		remoteID, _ := row["model_name"].(string)
		metadata, _ := row["metadata"].(map[string]any)
		if strings.TrimSpace(remoteID) != "" {
			metadataByID[remoteID] = metadata
		}
	}
	out := make([]RemoteProviderModel, 0, len(discovered))
	for _, item := range discovered {
		metadata := metadataByID[item.RemoteIdentifier]
		out = append(out, RemoteProviderModel{
			ProviderName:     provider.Name,
			ProviderType:     provider.Type,
			ModelName:        item.RemoteIdentifier,
			LocalName:        item.Name,
			DisplayName:      item.DisplayName,
			RemoteIdentifier: item.RemoteIdentifier,
			Metadata:         metadata,
		})
	}
	return out
}

func syncUpdateForDiscoveredModel(current schemas.Model, item DiscoveredModel, managedAt string) schemas.ModelUpdate {
	cfg := mergeAutoMetadata(current.Config, item, managedAt)
	delete(cfg, "last_missing_at")
	update := schemas.ModelUpdate{
		Config:           cfg,
		RemoteIdentifier: &item.RemoteIdentifier,
	}
	if current.DisplayName == nil && strings.TrimSpace(item.DisplayName) != "" {
		update.DisplayName = &item.DisplayName
	}
	if current.Description == nil && strings.TrimSpace(item.Description) != "" {
		update.Description = &item.Description
	}
	return update
}

func isAutoManagedModel(model schemas.Model) bool {
	return model.Config != nil && model.Config["managed_by"] == ModelAutoUpdateManager
}

func HasProviderRunError(result ModelUpdateResult) bool {
	for _, run := range result.ProviderRuns {
		if strings.TrimSpace(run.Error) != "" {
			return true
		}
	}
	return false
}
