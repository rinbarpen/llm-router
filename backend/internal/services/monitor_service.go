package services

import "github.com/rinbarpen/llm-router/backend/internal/db"

// MonitorService contains monitor-related utility logic shared by API handlers.
type MonitorService struct{}

func NewMonitorService() *MonitorService {
	return &MonitorService{}
}

// CalculateCost mirrors python behavior:
// - if both input/output prices exist, calculate separately
// - otherwise use unified cost_per_1k_tokens for total tokens.
func (s *MonitorService) CalculateCost(model db.Model, promptTokens *int64, completionTokens *int64) *float64 {
	if promptTokens == nil && completionTokens == nil {
		return nil
	}
	cfg := model.Config
	if cfg == nil {
		return nil
	}
	priceIn, hasIn := asFloat(cfg["cost_per_1k_tokens"])
	priceOut, hasOut := asFloat(cfg["cost_per_1k_completion_tokens"])
	if !hasIn && !hasOut {
		return nil
	}
	prompt := int64(0)
	if promptTokens != nil {
		prompt = *promptTokens
	}
	completion := int64(0)
	if completionTokens != nil {
		completion = *completionTokens
	}
	cost := 0.0
	if hasOut {
		cost += (float64(completion) / 1000.0) * priceOut
		if hasIn {
			cost += (float64(prompt) / 1000.0) * priceIn
		}
	} else if hasIn {
		cost += (float64(prompt+completion) / 1000.0) * priceIn
	}
	if cost <= 0 {
		return nil
	}
	v := round6(cost)
	return &v
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func round6(v float64) float64 {
	const scale = 1000000.0
	if v >= 0 {
		return float64(int64(v*scale+0.5)) / scale
	}
	return float64(int64(v*scale-0.5)) / scale
}
