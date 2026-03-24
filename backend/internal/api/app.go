package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/rinbarpen/llm-router/backend/internal/config"
	"github.com/rinbarpen/llm-router/backend/internal/db"
	"github.com/rinbarpen/llm-router/backend/internal/migrate"
	"github.com/rinbarpen/llm-router/backend/internal/services"
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
	handler := NewRouterWithOptions(catalog, RouterOptions{
		RequireAuth:      cfg.RequireAuth,
		AllowLocalNoAuth: cfg.AllowLocalNoAuth,
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
