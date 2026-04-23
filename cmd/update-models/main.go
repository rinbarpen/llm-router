package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rinbarpen/llm-router/src/config"
	"github.com/rinbarpen/llm-router/src/db"
	"github.com/rinbarpen/llm-router/src/migrate"
	"github.com/rinbarpen/llm-router/src/services"
)

type repeatFlag []string

func (f *repeatFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		*f = append(*f, value)
	}
	return nil
}

func main() {
	var providers repeatFlag
	var configPath string
	var sourceDir string
	var all bool
	var dryRun bool
	var jsonOut bool
	var timeout time.Duration

	flag.Var(&providers, "provider", "provider name or type to update; repeatable")
	flag.BoolVar(&all, "all", false, "update all providers")
	flag.StringVar(&configPath, "config", "", "router.toml path")
	flag.StringVar(&sourceDir, "source-dir", "", "model source JSON directory")
	flag.BoolVar(&dryRun, "dry-run", false, "compute changes without deleting DB rows or writing router.toml")
	flag.BoolVar(&jsonOut, "json", false, "write JSON result")
	flag.DurationVar(&timeout, "timeout", 10*time.Minute, "update timeout")
	flag.Parse()

	if all && len(providers) > 0 {
		exitWithError("use either --all or --provider, not both", jsonOut)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cfg, err := config.Load()
	if err != nil {
		exitWithError(err.Error(), jsonOut)
	}
	if strings.TrimSpace(configPath) != "" {
		cfg.ModelConfigPath = configPath
	}
	resolvedConfig, err := config.ResolveModelConfigPath(cfg.ModelConfigPath)
	if err != nil {
		exitWithError(err.Error(), jsonOut)
	}
	modelCfg, err := config.LoadRouterModelConfigFromTOML(resolvedConfig)
	if err != nil {
		exitWithError(err.Error(), jsonOut)
	}
	if strings.TrimSpace(sourceDir) == "" && modelCfg.ModelUpdates != nil {
		sourceDir = modelCfg.ModelUpdates.SourceDir
	}
	if strings.TrimSpace(sourceDir) == "" {
		sourceDir = "data/model_sources"
	}

	pool, err := db.Connect(ctx, cfg.PostgresDSN)
	if err != nil {
		exitWithError(err.Error(), jsonOut)
	}
	defer pool.Close()

	if err := migrate.Bootstrap(ctx, pool, cfg); err != nil {
		exitWithError(fmt.Sprintf("bootstrap migration failed: %v", err), jsonOut)
	}

	catalog := services.NewCatalogService(pool)
	if err := ensureProviderFiltersExist(ctx, catalog, providers); err != nil {
		exitWithError(err.Error(), jsonOut)
	}

	defaultNewModelActive := false
	writeRouterTOML := true
	if modelCfg.ModelUpdates != nil {
		defaultNewModelActive = modelCfg.ModelUpdates.DefaultNewModelActive
		writeRouterTOML = modelCfg.ModelUpdates.WriteRouterTOML
	}
	result, err := catalog.RunModelUpdateWithOptions(ctx, resolvedConfig, sourceDir, services.ModelUpdateOptions{
		DefaultNewModelActive: defaultNewModelActive,
		WriteRouterTOML:       writeRouterTOML,
		ProviderFilters:       providers,
		DryRun:                dryRun,
	})
	if err != nil {
		exitWithError(err.Error(), jsonOut)
	}
	if len(providers) > 0 && hasProviderError(result) {
		writeResult(result, jsonOut)
		os.Exit(1)
	}
	writeResult(result, jsonOut)
}

func ensureProviderFiltersExist(ctx context.Context, catalog *services.CatalogService, filters []string) error {
	if len(filters) == 0 {
		return nil
	}
	providers, err := catalog.ListProviders(ctx)
	if err != nil {
		return err
	}
	for _, filter := range filters {
		filter = strings.ToLower(strings.TrimSpace(filter))
		found := false
		for _, provider := range providers {
			if filter == strings.ToLower(strings.TrimSpace(provider.Name)) || filter == strings.ToLower(strings.TrimSpace(provider.Type)) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("provider %q not found by name or type", filter)
		}
	}
	return nil
}

func hasProviderError(result services.ModelUpdateResult) bool {
	for _, run := range result.ProviderRuns {
		if strings.TrimSpace(run.Error) != "" {
			return true
		}
	}
	return false
}

func writeResult(result services.ModelUpdateResult, jsonOut bool) {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return
	}
	for _, run := range result.ProviderRuns {
		fmt.Printf("%s: added=%d updated=%d deleted=%d skipped=%d",
			run.ProviderName, len(run.Added), len(run.Updated), len(run.Deleted), len(run.Skipped))
		if run.Error != "" {
			fmt.Printf(" error=%q", run.Error)
		}
		if run.BackupPath != "" {
			fmt.Printf(" backup_path=%s", run.BackupPath)
		}
		fmt.Println()
	}
	if result.BackupPath != "" {
		fmt.Printf("backup_path=%s\n", result.BackupPath)
	}
}

func exitWithError(message string, jsonOut bool) {
	if jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"error": message})
	} else {
		slog.Error(message)
	}
	os.Exit(1)
}
