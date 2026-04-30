package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rinbarpen/llm-router/src/api"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := api.Run(ctx); err != nil {
		slog.Error("llm-router exited", slog.Any("error", err))
		os.Exit(1)
	}
}
