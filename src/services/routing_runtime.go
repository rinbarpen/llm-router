package services

import (
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rinbarpen/llm-router/src/config"
)

const (
	loadBalanceRoundRobin  = "round_robin"
	loadBalanceWeighted    = "weighted"
	loadBalanceLeastFailed = "least_failure"
)

type routingRuntime struct {
	mu sync.RWMutex

	strategy          string
	channelFallback   []string
	providerWeights   map[string]int64
	dynamicEnabled    bool
	dynamicMinSamples int64

	circuitEnabled     bool
	circuitThreshold   int64
	circuitCooldown    time.Duration
	circuitHalfOpenMax int64
	circuitState       map[string]*providerCircuitState

	rrCounter uint64
	metrics   map[string]*providerLoadMetrics
}

type providerCircuitState struct {
	consecutiveFailures int64
	openUntil           time.Time
	halfOpenRemaining   int64
}

type providerLoadMetrics struct {
	inflight      int64
	successes     int64
	failures      int64
	totalLatency  int64
	lastUpdatedAt time.Time
}

type channelLoadSnapshot struct {
	Strategy    string                 `json:"strategy"`
	GeneratedAt time.Time              `json:"generated_at"`
	Providers   []providerLoadSnapshot `json:"providers"`
}

type providerLoadSnapshot struct {
	ProviderName        string     `json:"provider_name"`
	Inflight            int64      `json:"inflight"`
	Successes           int64      `json:"successes"`
	Failures            int64      `json:"failures"`
	FailureRate         float64    `json:"failure_rate"`
	AverageLatencyMS    float64    `json:"average_latency_ms"`
	CircuitOpen         bool       `json:"circuit_open"`
	CircuitOpenUntil    *time.Time `json:"circuit_open_until,omitempty"`
	ConsecutiveFailures int64      `json:"consecutive_failures"`
}

func newRoutingRuntime() *routingRuntime {
	return &routingRuntime{
		strategy:           loadBalanceRoundRobin,
		channelFallback:    nil,
		providerWeights:    map[string]int64{},
		dynamicEnabled:     false,
		dynamicMinSamples:  20,
		circuitEnabled:     false,
		circuitThreshold:   3,
		circuitCooldown:    30 * time.Second,
		circuitHalfOpenMax: 1,
		circuitState:       map[string]*providerCircuitState{},
		metrics:            map[string]*providerLoadMetrics{},
	}
}

func (r *routingRuntime) applyConfig(cfg *config.RoutingConfig) {
	if cfg == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	strategy := strings.ToLower(strings.TrimSpace(cfg.LoadBalanceStrategy))
	switch strategy {
	case loadBalanceRoundRobin, loadBalanceWeighted, loadBalanceLeastFailed:
		r.strategy = strategy
	default:
		r.strategy = loadBalanceRoundRobin
	}

	r.channelFallback = make([]string, 0, len(cfg.ChannelFallback))
	seen := map[string]struct{}{}
	for _, p := range cfg.ChannelFallback {
		name := strings.TrimSpace(p)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		r.channelFallback = append(r.channelFallback, name)
	}

	r.providerWeights = map[string]int64{}
	for provider, weight := range cfg.ProviderWeights {
		name := strings.TrimSpace(provider)
		if name == "" {
			continue
		}
		if weight <= 0 {
			weight = 1
		}
		r.providerWeights[name] = weight
	}
	if cfg.DynamicWeights != nil {
		r.dynamicEnabled = cfg.DynamicWeights.Enabled
		if cfg.DynamicWeights.MinSamples > 0 {
			r.dynamicMinSamples = cfg.DynamicWeights.MinSamples
		}
	}

	if cfg.CircuitBreaker != nil {
		r.circuitEnabled = cfg.CircuitBreaker.Enabled
		if cfg.CircuitBreaker.FailureThreshold > 0 {
			r.circuitThreshold = cfg.CircuitBreaker.FailureThreshold
		}
		if cfg.CircuitBreaker.CooldownSeconds > 0 {
			r.circuitCooldown = time.Duration(cfg.CircuitBreaker.CooldownSeconds) * time.Second
		}
		if cfg.CircuitBreaker.HalfOpenMaxRequests > 0 {
			r.circuitHalfOpenMax = cfg.CircuitBreaker.HalfOpenMaxRequests
		}
	}
}

func (r *routingRuntime) begin(provider string) (func(success bool, duration time.Duration), bool) {
	now := time.Now()
	allow := r.allowProvider(provider, now)
	if !allow {
		return func(bool, time.Duration) {}, false
	}
	r.mu.Lock()
	m := r.metrics[provider]
	if m == nil {
		m = &providerLoadMetrics{}
		r.metrics[provider] = m
	}
	m.inflight++
	m.lastUpdatedAt = now
	r.mu.Unlock()

	return func(success bool, duration time.Duration) {
		r.mu.Lock()
		defer r.mu.Unlock()
		stat := r.metrics[provider]
		if stat == nil {
			stat = &providerLoadMetrics{}
			r.metrics[provider] = stat
		}
		if stat.inflight > 0 {
			stat.inflight--
		}
		if success {
			stat.successes++
			if r.circuitEnabled {
				state := r.getCircuitStateLocked(provider)
				state.consecutiveFailures = 0
				state.openUntil = time.Time{}
				state.halfOpenRemaining = 0
			}
		} else {
			stat.failures++
			if r.circuitEnabled {
				state := r.getCircuitStateLocked(provider)
				state.consecutiveFailures++
				if state.consecutiveFailures >= r.circuitThreshold {
					state.openUntil = time.Now().Add(r.circuitCooldown)
					state.halfOpenRemaining = r.circuitHalfOpenMax
				}
			}
		}
		stat.totalLatency += duration.Milliseconds()
		stat.lastUpdatedAt = time.Now()
	}, true
}

func (r *routingRuntime) allowProvider(provider string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.circuitEnabled {
		return true
	}
	state := r.getCircuitStateLocked(provider)
	if state.openUntil.IsZero() {
		return true
	}
	if now.Before(state.openUntil) {
		return false
	}
	if state.halfOpenRemaining <= 0 {
		return false
	}
	state.halfOpenRemaining--
	return true
}

func (r *routingRuntime) getCircuitStateLocked(provider string) *providerCircuitState {
	state := r.circuitState[provider]
	if state == nil {
		state = &providerCircuitState{}
		r.circuitState[provider] = state
	}
	return state
}

func (r *routingRuntime) orderCandidates(modelKey string, candidates []chatTarget) []chatTarget {
	if len(candidates) <= 1 {
		return candidates
	}
	r.mu.RLock()
	strategy := r.strategy
	weights := map[string]int64{}
	for k, v := range r.providerWeights {
		weights[k] = v
	}
	metrics := map[string]providerLoadMetrics{}
	for k, v := range r.metrics {
		if v == nil {
			continue
		}
		metrics[k] = *v
	}
	r.mu.RUnlock()

	out := make([]chatTarget, len(candidates))
	copy(out, candidates)
	switch strategy {
	case loadBalanceWeighted:
		if r.dynamicEnabled {
			weights = r.adjustWeights(weights, metrics)
		}
		return r.orderWeighted(out, weights)
	case loadBalanceLeastFailed:
		sort.SliceStable(out, func(i, j int) bool {
			a := metrics[out[i].ProviderName]
			b := metrics[out[j].ProviderName]
			rateA := failureRate(a.failures, a.successes)
			rateB := failureRate(b.failures, b.successes)
			if math.Abs(rateA-rateB) > 1e-9 {
				return rateA < rateB
			}
			if a.inflight != b.inflight {
				return a.inflight < b.inflight
			}
			return out[i].ProviderName < out[j].ProviderName
		})
		return out
	default:
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].ProviderName < out[j].ProviderName
		})
		counter := atomic.AddUint64(&r.rrCounter, 1)
		if modelKey != "" {
			counter += uint64(len(modelKey))
		}
		shift := int(counter % uint64(len(out)))
		rotated := append(out[shift:], out[:shift]...)
		return rotated
	}
}

func (r *routingRuntime) adjustWeights(base map[string]int64, metrics map[string]providerLoadMetrics) map[string]int64 {
	out := map[string]int64{}
	for provider, weight := range base {
		if weight <= 0 {
			weight = 1
		}
		m := metrics[provider]
		total := m.successes + m.failures
		if total < r.dynamicMinSamples {
			out[provider] = weight
			continue
		}
		fail := failureRate(m.failures, m.successes)
		latencyPenalty := 1.0
		if m.successes+m.failures > 0 {
			avg := float64(m.totalLatency) / float64(m.successes+m.failures)
			latencyPenalty += avg / 1000.0
		}
		score := 1.0 + fail*10.0
		score *= latencyPenalty
		adjusted := int64(float64(weight) / score)
		if adjusted <= 0 {
			adjusted = 1
		}
		out[provider] = adjusted
	}
	for provider := range metrics {
		if _, ok := out[provider]; ok {
			continue
		}
		out[provider] = 1
	}
	return out
}

func (r *routingRuntime) orderWeighted(candidates []chatTarget, weights map[string]int64) []chatTarget {
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].ProviderName < candidates[j].ProviderName
	})
	total := int64(0)
	for _, c := range candidates {
		w := weights[c.ProviderName]
		if w <= 0 {
			w = 1
		}
		total += w
	}
	if total <= 0 {
		return candidates
	}

	cursor := int64(atomic.AddUint64(&r.rrCounter, 1) % uint64(total))
	pick := 0
	for i, c := range candidates {
		w := weights[c.ProviderName]
		if w <= 0 {
			w = 1
		}
		if cursor < w {
			pick = i
			break
		}
		cursor -= w
	}
	ordered := make([]chatTarget, 0, len(candidates))
	ordered = append(ordered, candidates[pick])
	for i, c := range candidates {
		if i == pick {
			continue
		}
		ordered = append(ordered, c)
	}
	return ordered
}

func (r *routingRuntime) fallbackProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.channelFallback))
	copy(out, r.channelFallback)
	return out
}

func failureRate(failures, successes int64) float64 {
	total := failures + successes
	if total <= 0 {
		return 0
	}
	return float64(failures) / float64(total)
}

func (r *routingRuntime) snapshot() channelLoadSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := channelLoadSnapshot{
		Strategy:    r.strategy,
		GeneratedAt: time.Now().UTC(),
		Providers:   make([]providerLoadSnapshot, 0, len(r.metrics)),
	}
	for provider, metric := range r.metrics {
		if metric == nil {
			continue
		}
		avgLatency := 0.0
		totalCalls := metric.successes + metric.failures
		if totalCalls > 0 {
			avgLatency = float64(metric.totalLatency) / float64(totalCalls)
		}
		state := r.circuitState[provider]
		var openUntil *time.Time
		circuitOpen := false
		consecutiveFailures := int64(0)
		if state != nil {
			consecutiveFailures = state.consecutiveFailures
			if !state.openUntil.IsZero() && time.Now().Before(state.openUntil) {
				circuitOpen = true
				t := state.openUntil.UTC()
				openUntil = &t
			}
		}
		out.Providers = append(out.Providers, providerLoadSnapshot{
			ProviderName:        provider,
			Inflight:            metric.inflight,
			Successes:           metric.successes,
			Failures:            metric.failures,
			FailureRate:         failureRate(metric.failures, metric.successes),
			AverageLatencyMS:    avgLatency,
			CircuitOpen:         circuitOpen,
			CircuitOpenUntil:    openUntil,
			ConsecutiveFailures: consecutiveFailures,
		})
	}
	sort.SliceStable(out.Providers, func(i, j int) bool {
		return out.Providers[i].ProviderName < out.Providers[j].ProviderName
	})
	return out
}
