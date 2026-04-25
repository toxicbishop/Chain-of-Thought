// Package metrics provides Prometheus-compatible instrumentation for the
// CoT backend. It tracks agent execution times, request counts, error rates,
// and cache hit ratios — giving full observability without external tracing.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ── Counters & Histograms ───────────────────────────────────────────────────

// Counter is a monotonically increasing counter with optional labels.
type Counter struct {
	mu     sync.Mutex
	values map[string]*atomic.Int64 // label-hash → count
}

func NewCounter() *Counter {
	return &Counter{values: make(map[string]*atomic.Int64)}
}

func (c *Counter) Inc(labels ...string) {
	key := strings.Join(labels, "|")
	c.mu.Lock()
	v, ok := c.values[key]
	if !ok {
		v = &atomic.Int64{}
		c.values[key] = v
	}
	c.mu.Unlock()
	v.Add(1)
}

func (c *Counter) snapshot() map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.values))
	for k, v := range c.values {
		out[k] = v.Load()
	}
	return out
}

// Histogram tracks observation counts and sum for computing averages.
type Histogram struct {
	mu  sync.Mutex
	obs map[string]*histBucket
}

type histBucket struct {
	count int64
	sum   float64
}

func NewHistogram() *Histogram {
	return &Histogram{obs: make(map[string]*histBucket)}
}

func (h *Histogram) Observe(val float64, labels ...string) {
	key := strings.Join(labels, "|")
	h.mu.Lock()
	b, ok := h.obs[key]
	if !ok {
		b = &histBucket{}
		h.obs[key] = b
	}
	b.count++
	b.sum += val
	h.mu.Unlock()
}

func (h *Histogram) snapshot() map[string]histBucket {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]histBucket, len(h.obs))
	for k, v := range h.obs {
		out[k] = *v
	}
	return out
}

// ── Global Registry ─────────────────────────────────────────────────────────

var (
	// RequestsTotal counts HTTP requests by method and path.
	RequestsTotal = NewCounter()

	// AgentRunsTotal counts agent executions by agent name and status.
	AgentRunsTotal = NewCounter()

	// AgentDurationSeconds tracks agent execution latency by agent name.
	AgentDurationSeconds = NewHistogram()

	// CacheHitsTotal counts cache lookups by result (hit/miss).
	CacheHitsTotal = NewCounter()

	// LLMErrorsTotal counts LLM call failures by agent name.
	LLMErrorsTotal = NewCounter()

	// StreamSessionsActive tracks currently open SSE connections.
	StreamSessionsActive atomic.Int64
)

// RecordAgentRun is a convenience helper called by the executor.
func RecordAgentRun(agentName, status string, duration time.Duration) {
	AgentRunsTotal.Inc(agentName, status)
	AgentDurationSeconds.Observe(duration.Seconds(), agentName)
	if status == "failed" {
		LLMErrorsTotal.Inc(agentName)
	}
}

// ── /metrics HTTP Handler ───────────────────────────────────────────────────

// Handler returns an HTTP handler that renders all metrics in Prometheus
// text exposition format.
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		var sb strings.Builder

		// Requests
		sb.WriteString("# HELP cot_requests_total Total HTTP requests.\n")
		sb.WriteString("# TYPE cot_requests_total counter\n")
		writeCounter(&sb, "cot_requests_total", RequestsTotal, []string{"method", "path"})

		// Agent runs
		sb.WriteString("# HELP cot_agent_runs_total Total agent executions.\n")
		sb.WriteString("# TYPE cot_agent_runs_total counter\n")
		writeCounter(&sb, "cot_agent_runs_total", AgentRunsTotal, []string{"agent", "status"})

		// Agent duration
		sb.WriteString("# HELP cot_agent_duration_seconds Agent execution latency.\n")
		sb.WriteString("# TYPE cot_agent_duration_seconds summary\n")
		writeHistogram(&sb, "cot_agent_duration_seconds", AgentDurationSeconds, []string{"agent"})

		// Cache
		sb.WriteString("# HELP cot_cache_hits_total Cache lookup results.\n")
		sb.WriteString("# TYPE cot_cache_hits_total counter\n")
		writeCounter(&sb, "cot_cache_hits_total", CacheHitsTotal, []string{"result"})

		// LLM errors
		sb.WriteString("# HELP cot_llm_errors_total LLM call failures.\n")
		sb.WriteString("# TYPE cot_llm_errors_total counter\n")
		writeCounter(&sb, "cot_llm_errors_total", LLMErrorsTotal, []string{"agent"})

		// Active streams
		sb.WriteString("# HELP cot_stream_sessions_active Currently open SSE connections.\n")
		sb.WriteString("# TYPE cot_stream_sessions_active gauge\n")
		fmt.Fprintf(&sb, "cot_stream_sessions_active %d\n", StreamSessionsActive.Load())

		w.Write([]byte(sb.String()))
	}
}

func writeCounter(sb *strings.Builder, name string, c *Counter, labelNames []string) {
	snap := c.snapshot()
	keys := sortedKeys(snap)
	for _, k := range keys {
		labels := formatLabels(labelNames, strings.Split(k, "|"))
		fmt.Fprintf(sb, "%s{%s} %d\n", name, labels, snap[k])
	}
}

func writeHistogram(sb *strings.Builder, name string, h *Histogram, labelNames []string) {
	snap := h.snapshot()
	for k, b := range snap {
		labels := formatLabels(labelNames, strings.Split(k, "|"))
		fmt.Fprintf(sb, "%s_count{%s} %d\n", name, labels, b.count)
		fmt.Fprintf(sb, "%s_sum{%s} %.4f\n", name, labels, b.sum)
	}
}

func formatLabels(names, values []string) string {
	parts := make([]string, 0, len(names))
	for i, n := range names {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		parts = append(parts, fmt.Sprintf(`%s="%s"`, n, v))
	}
	return strings.Join(parts, ",")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
