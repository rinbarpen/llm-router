package services

import (
	"context"
	"log/slog"
	"testing"

	routerlog "github.com/rinbarpen/llm-router/src/logging"
)

func TestRecordChatRouteDecisionUsesProviderHintMode(t *testing.T) {
	ctx, state := routerlog.ContextWithRequestLogState(context.Background())
	target := chatTarget{ProviderName: "openai", ModelName: "gpt-4o"}

	recordChatRouteDecision(ctx, target, "openai", "openai/gpt-4o", false)
	recordChatRouteResult(ctx, true)

	attrs := attrsByKey(state.Attrs())
	if got := attrs["provider"]; got != "openai" {
		t.Fatalf("provider = %v, want openai", got)
	}
	if got := attrs["model"]; got != "gpt-4o" {
		t.Fatalf("model = %v, want gpt-4o", got)
	}
	if got := attrs["routing_mode"]; got != "provider_hint" {
		t.Fatalf("routing_mode = %v, want provider_hint", got)
	}
	if got := attrs["stream"]; got != false {
		t.Fatalf("stream = %v, want false", got)
	}
	if got := attrs["result"]; got != "success" {
		t.Fatalf("result = %v, want success", got)
	}
}

func TestRecordChatRouteDecisionUsesRemoteIdentifierMode(t *testing.T) {
	ctx, state := routerlog.ContextWithRequestLogState(context.Background())
	remote := "gpt-4o-mini-remote"
	target := chatTarget{
		ProviderName:     "openrouter",
		ModelName:        "gpt-4o-mini",
		RemoteIdentifier: &remote,
	}

	recordChatRouteDecision(ctx, target, "", "gpt-4o-mini-remote", true)
	recordChatRouteResult(ctx, false)

	attrs := attrsByKey(state.Attrs())
	if got := attrs["routing_mode"]; got != "remote_identifier" {
		t.Fatalf("routing_mode = %v, want remote_identifier", got)
	}
	if got := attrs["stream"]; got != true {
		t.Fatalf("stream = %v, want true", got)
	}
	if got := attrs["result"]; got != "fail" {
		t.Fatalf("result = %v, want fail", got)
	}
}

func attrsByKey(attrs []slog.Attr) map[string]any {
	out := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		out[attr.Key] = attr.Value.Any()
	}
	return out
}
