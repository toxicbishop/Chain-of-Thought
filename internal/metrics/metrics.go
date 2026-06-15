package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
	// RequestsTotal counts HTTP requests by method and path.
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cot_requests_total",
		Help: "Total HTTP requests.",
	}, []string{"method", "path"})

	// AgentRunsTotal counts agent executions by agent name and status.
	AgentRunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cot_agent_runs_total",
		Help: "Total agent executions.",
	}, []string{"agent", "status"})

	// AgentDurationSeconds tracks agent execution latency by agent name.
	AgentDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cot_agent_duration_seconds",
		Help:    "Agent execution latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"agent"})

	// CacheHitsTotal counts cache lookups by result (hit/miss).
	CacheHitsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cot_cache_hits_total",
		Help: "Cache lookup results.",
	}, []string{"result"})

	// LLMErrorsTotal counts LLM call failures by agent name.
	LLMErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cot_llm_errors_total",
		Help: "LLM call failures.",
	}, []string{"agent"})

	// StreamSessionsActive tracks currently open SSE connections.
	StreamSessionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cot_stream_sessions_active",
		Help: "Currently open SSE connections.",
	})
)

// RecordAgentRun is a convenience helper called by the executor.
func RecordAgentRun(agentName, status string, duration time.Duration) {
	AgentRunsTotal.WithLabelValues(agentName, status).Inc()
	AgentDurationSeconds.WithLabelValues(agentName).Observe(duration.Seconds())
	if status == "failed" {
		LLMErrorsTotal.WithLabelValues(agentName).Inc()
	}
}

// Handler returns an HTTP handler that renders all metrics in Prometheus format.
func Handler() http.Handler {
	return promhttp.Handler()
}
