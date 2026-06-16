package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	AgentDurationSeconds = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name: "cot_agent_duration_seconds",
		Help: "Agent execution latency.",
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

	// KafkaConsumerLag tracks the number of unprocessed messages per topic.
	KafkaConsumerLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cot_kafka_consumer_lag",
		Help: "Number of unprocessed messages between latest offset and last committed offset, per topic.",
	}, []string{"topic"})

	// KafkaDLQTotal counts messages routed to the dead letter queue per topic.
	KafkaDLQTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cot_kafka_dlq_total",
		Help: "Total messages moved to the DLQ after exhausting retries.",
	}, []string{"topic"})

	// KafkaMessagesProcessed counts successfully processed messages per topic.
	KafkaMessagesProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cot_kafka_messages_processed_total",
		Help: "Total messages successfully processed by the consumer.",
	}, []string{"topic"})

	// KafkaRetries counts retry attempts per topic.
	KafkaRetries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cot_kafka_retries_total",
		Help: "Total retry attempts before success or DLQ routing.",
	}, []string{"topic"})

	// RateLimitChecks counts rate limit decisions by path and result.
	// result label values: "allowed", "rejected", "error"
	// path label is the request URL path, giving per-endpoint visibility.
	RateLimitChecks = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cot_rate_limit_checks_total",
		Help: "Total rate limit evaluations, labelled by endpoint path and result (allowed/rejected/error).",
	}, []string{"path", "result"})
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
