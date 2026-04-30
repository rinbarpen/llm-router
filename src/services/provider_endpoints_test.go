package services

import (
	"testing"
	"time"
)

func TestProviderEndpointRuntimeOrdersHealthyLowLatencyEndpointsFirst(t *testing.T) {
	rt := newProviderEndpointRuntime()
	settings := map[string]any{
		"api_base_urls": []any{
			"https://api.vveai.com",
			"https://api.gpt.ge",
			"https://api.v3.cm",
		},
		"latency_degrade_threshold_ms": float64(3000),
	}

	rt.finish("third-party", "https://api.gpt.ge", true, 5*time.Second, true)
	rt.finish("third-party", "https://api.v3.cm", true, 1200*time.Millisecond, true)
	rt.finish("third-party", "https://api.vveai.com", true, 800*time.Millisecond, true)

	ordered := rt.order("third-party", nil, settings)
	if len(ordered) != 3 {
		t.Fatalf("ordered count = %d, want 3", len(ordered))
	}
	if ordered[0] != "https://api.vveai.com" {
		t.Fatalf("first endpoint = %s, want fastest healthy endpoint", ordered[0])
	}
	if ordered[2] != "https://api.gpt.ge" {
		t.Fatalf("slow endpoint should be deprioritized, got order=%v", ordered)
	}
}

func TestProviderEndpointRuntimeSkipsCoolingDownEndpointThenRecovers(t *testing.T) {
	rt := newProviderEndpointRuntime()
	settings := map[string]any{
		"api_base_urls": []any{
			"https://api.vveai.com",
			"https://api.gpt.ge",
		},
		"cooldown_seconds": float64(1),
	}

	rt.finish("third-party", "https://api.vveai.com", false, 100*time.Millisecond, true)

	ordered := rt.order("third-party", nil, settings)
	if len(ordered) != 2 {
		t.Fatalf("ordered count = %d, want 2", len(ordered))
	}
	if ordered[0] != "https://api.gpt.ge" {
		t.Fatalf("cooling down endpoint should not be first, got=%v", ordered)
	}

	time.Sleep(1100 * time.Millisecond)
	ordered = rt.order("third-party", nil, settings)
	if len(ordered) != 2 {
		t.Fatalf("ordered count after cooldown = %d, want 2", len(ordered))
	}
	foundPrimary := false
	for _, item := range ordered {
		if item == "https://api.vveai.com" {
			foundPrimary = true
			break
		}
	}
	if !foundPrimary {
		t.Fatalf("endpoint should return after cooldown, got=%v", ordered)
	}
}
