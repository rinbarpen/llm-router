package services

import (
	"context"
	"log/slog"
	"strings"

	routerlog "github.com/rinbarpen/llm-router/src/logging"
)

func recordChatRouteDecision(ctx context.Context, target chatTarget, providerHint string, requestedModel string, stream bool) {
	routerlog.WithAttrs(ctx,
		slog.String("provider", target.ProviderName),
		slog.String("model", target.ModelName),
		slog.String("routing_mode", chatRoutingMode(target, providerHint, requestedModel)),
		slog.Bool("stream", stream),
	)
}

func recordChatRouteResult(ctx context.Context, success bool) {
	result := "fail"
	if success {
		result = "success"
	}
	routerlog.WithAttrs(ctx, slog.String("result", result))
}

func chatRoutingMode(target chatTarget, providerHint string, requestedModel string) string {
	if strings.TrimSpace(providerHint) != "" {
		return "provider_hint"
	}
	if target.RemoteIdentifier != nil && strings.TrimSpace(*target.RemoteIdentifier) != "" && strings.TrimSpace(*target.RemoteIdentifier) == strings.TrimSpace(requestedModel) {
		return "remote_identifier"
	}
	if parts := strings.SplitN(strings.TrimSpace(requestedModel), "/", 2); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return "model_path"
	}
	return "model_name"
}
