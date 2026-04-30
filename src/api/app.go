package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rinbarpen/llm-router/src/config"
	"github.com/rinbarpen/llm-router/src/db"
	"github.com/rinbarpen/llm-router/src/logging"
	"github.com/rinbarpen/llm-router/src/migrate"
	"github.com/rinbarpen/llm-router/src/services"
)

func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger, closeLogs, err := logging.NewLogger(logging.Options{
		Level:         cfg.Logging.Level,
		Format:        cfg.Logging.Format,
		StdoutEnabled: cfg.Logging.StdoutEnabled,
		FilePath:      cfg.Logging.FilePath,
	})
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = closeLogs() }()
	slog.SetDefault(logger)

	pool, err := db.Connect(ctx, cfg.SQLitePath)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := migrate.Bootstrap(ctx, pool, cfg); err != nil {
		return fmt.Errorf("bootstrap migration failed: %w", err)
	}

	catalog := services.NewCatalogService(pool)
	var modelUpdates *config.ModelUpdatesConfig
	var resolvedModelConfigPath string
	if resolved, resolveErr := config.ResolveModelConfigPath(cfg.ModelConfigPath); resolveErr == nil {
		resolvedModelConfigPath = resolved
		if modelCfg, loadErr := config.LoadRouterModelConfigFromTOML(resolved); loadErr == nil {
			catalog.ApplyRoutingConfig(modelCfg.Routing)
			modelUpdates = modelCfg.ModelUpdates
		} else {
			logger.Warn("llm-router: skip routing policy load", slog.String("path", resolved), slog.Any("error", loadErr))
		}
	}
	if modelUpdates != nil && modelUpdates.Enabled {
		startModelUpdateScheduler(ctx, catalog, resolvedModelConfigPath, *modelUpdates, logger)
	}
	handler := NewRouterWithOptions(catalog, RouterOptions{
		RequireAuth:         cfg.RequireAuth,
		AllowLocalNoAuth:    cfg.AllowLocalNoAuth,
		ModelConfigHintPath: cfg.ModelConfigPath,
		Logger:              logger,
	})
	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	logger.Info("llm-router-go listening", slog.String("addr", server.Addr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func startModelUpdateScheduler(ctx context.Context, catalog *services.CatalogService, configPath string, cfg config.ModelUpdatesConfig, logger *slog.Logger) {
	if cfg.IntervalHours <= 0 {
		cfg.IntervalHours = 24
	}
	if strings.TrimSpace(cfg.SourceDir) == "" {
		cfg.SourceDir = "data/model_sources"
	}
	delay := time.Duration(cfg.StartupDelaySeconds * float64(time.Second))
	if delay < 0 {
		delay = 0
	}
	runOnce := func(reason string) {
		go func() {
			runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()
			_ = configPath
			_ = cfg.SourceDir
			_ = cfg.WriteRouterTOML
			result, err := catalog.SyncAllProviderModelsFromRemote(runCtx, services.ProviderModelSyncOptions{
				DefaultNewModelActive: cfg.DefaultNewModelActive,
			})
			if err != nil {
				logger.Warn("llm-router: model auto-update failed", slog.String("reason", reason), slog.Any("error", err))
			} else if services.HasProviderRunError(result) {
				logger.Warn("llm-router: model auto-update completed with provider errors", slog.String("reason", reason))
			}
		}()
	}
	if cfg.StartupSync {
		go func() {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				runOnce("startup")
			}
		}()
	}
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.IntervalHours) * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runOnce("scheduled")
			}
		}
	}()
}
