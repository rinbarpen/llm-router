package services

import (
	"time"

	"github.com/rinbarpen/llm-router/src/config"
)

func (s *CatalogService) ApplyRoutingConfig(cfg *config.RoutingConfig) {
	if s == nil || s.routeRT == nil {
		return
	}
	s.routeRT.applyConfig(cfg)
}

func (s *CatalogService) GetChannelLoadSnapshot() map[string]any {
	if s == nil || s.routeRT == nil {
		return map[string]any{
			"strategy":  loadBalanceRoundRobin,
			"providers": []any{},
		}
	}
	snapshot := s.routeRT.snapshot()
	providers := make([]map[string]any, 0, len(snapshot.Providers))
	for _, item := range snapshot.Providers {
		row := map[string]any{
			"provider_name":        item.ProviderName,
			"inflight":             item.Inflight,
			"successes":            item.Successes,
			"failures":             item.Failures,
			"failure_rate":         item.FailureRate,
			"average_latency_ms":   item.AverageLatencyMS,
			"circuit_open":         item.CircuitOpen,
			"consecutive_failures": item.ConsecutiveFailures,
		}
		if item.CircuitOpenUntil != nil {
			row["circuit_open_until"] = item.CircuitOpenUntil.Format(time.RFC3339)
		}
		providers = append(providers, row)
	}
	return map[string]any{
		"strategy":     snapshot.Strategy,
		"generated_at": snapshot.GeneratedAt.Format(time.RFC3339),
		"providers":    providers,
	}
}
