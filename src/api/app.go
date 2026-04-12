package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/rinbarpen/llm-router/src/config"
	"github.com/rinbarpen/llm-router/src/db"
	"github.com/rinbarpen/llm-router/src/migrate"
	"github.com/rinbarpen/llm-router/src/services"
)

func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := db.Connect(ctx, cfg.PostgresDSN)
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
			log.Printf("llm-router: skip routing policy load from %s: %v", resolved, loadErr)
		}
	}
	if modelUpdates != nil && modelUpdates.Enabled {
		startModelUpdateScheduler(ctx, catalog, resolvedModelConfigPath, *modelUpdates)
	}
	handler := NewRouterWithOptions(catalog, RouterOptions{
		RequireAuth:         cfg.RequireAuth,
		AllowLocalNoAuth:    cfg.AllowLocalNoAuth,
		ModelConfigHintPath: cfg.ModelConfigPath,
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

	log.Printf("llm-router-go listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func startModelUpdateScheduler(ctx context.Context, catalog *services.CatalogService, configPath string, cfg config.ModelUpdatesConfig) {
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
			if _, err := catalog.RunModelUpdate(runCtx, configPath, cfg.SourceDir, cfg.WriteRouterTOML, cfg.DefaultNewModelActive); err != nil {
				log.Printf("llm-router: model auto-update failed (%s): %v", reason, err)
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
