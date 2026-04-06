package services

import (
	"testing"
	"time"
)

func TestRoutingRuntimeRoundRobinOrder(t *testing.T) {
	rt := newRoutingRuntime()
	rt.strategy = loadBalanceRoundRobin
	candidates := []chatTarget{
		{ProviderName: "a", ModelName: "m"},
		{ProviderName: "b", ModelName: "m"},
		{ProviderName: "c", ModelName: "m"},
	}

	first := rt.orderCandidates("m", candidates)
	second := rt.orderCandidates("m", candidates)
	if len(first) != 3 || len(second) != 3 {
		t.Fatalf("unexpected candidate size: first=%d second=%d", len(first), len(second))
	}
	if first[0].ProviderName == second[0].ProviderName {
		t.Fatalf("round robin should rotate first provider, got=%s", first[0].ProviderName)
	}
}

func TestRoutingRuntimeWeightedOrder(t *testing.T) {
	rt := newRoutingRuntime()
	rt.strategy = loadBalanceWeighted
	rt.providerWeights["heavy"] = 10
	rt.providerWeights["light"] = 1
	candidates := []chatTarget{
		{ProviderName: "heavy", ModelName: "m"},
		{ProviderName: "light", ModelName: "m"},
	}

	pickedHeavy := 0
	total := 20
	for i := 0; i < total; i++ {
		ordered := rt.orderCandidates("m", candidates)
		if ordered[0].ProviderName == "heavy" {
			pickedHeavy++
		}
	}
	if pickedHeavy <= total/2 {
		t.Fatalf("weighted ordering did not prioritize heavy provider, picked=%d total=%d", pickedHeavy, total)
	}
}

func TestRoutingRuntimeLeastFailureOrder(t *testing.T) {
	rt := newRoutingRuntime()
	rt.strategy = loadBalanceLeastFailed
	rt.metrics["bad"] = &providerLoadMetrics{failures: 10, successes: 1}
	rt.metrics["good"] = &providerLoadMetrics{failures: 1, successes: 10}

	ordered := rt.orderCandidates("m", []chatTarget{
		{ProviderName: "bad", ModelName: "m"},
		{ProviderName: "good", ModelName: "m"},
	})
	if ordered[0].ProviderName != "good" {
		t.Fatalf("least_failure should prioritize low-failure provider, got=%s", ordered[0].ProviderName)
	}
}

func TestRoutingRuntimeCircuitBreaker(t *testing.T) {
	rt := newRoutingRuntime()
	rt.circuitEnabled = true
	rt.circuitThreshold = 2
	rt.circuitCooldown = 50 * time.Millisecond
	rt.circuitHalfOpenMax = 1

	end1, ok := rt.begin("provider-a")
	if !ok {
		t.Fatalf("first call should be allowed")
	}
	end1(false, 10*time.Millisecond)

	end2, ok := rt.begin("provider-a")
	if !ok {
		t.Fatalf("second call should be allowed before threshold reached")
	}
	end2(false, 10*time.Millisecond)

	_, ok = rt.begin("provider-a")
	if ok {
		t.Fatalf("circuit should be open after threshold failures")
	}

	time.Sleep(60 * time.Millisecond)
	_, ok = rt.begin("provider-a")
	if !ok {
		t.Fatalf("circuit should allow half-open probe after cooldown")
	}
}
