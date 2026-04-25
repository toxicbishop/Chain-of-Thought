// Package kafka provides the KafkaService that wires together the producer,
// consumer, and application pipeline to publish reasoning events and process
// async query requests from Kafka topics.
package kafka

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"cot-backend/internal/transformer"
)

// Service is the top-level Kafka integration object. It composes a producer
// and a consumer, and exposes high-level methods for the rest of the application.
type Service struct {
	producer *Producer
	consumer *Consumer
	enabled  bool
}

// NewService constructs a KafkaService from a comma-separated broker list.
// If brokerList is empty the service starts in disabled mode — all publish
// calls become no-ops so the application works without a running Kafka cluster.
func NewService(brokerList string) *Service {
	if brokerList == "" {
		log.Println("[kafka] KAFKA_BROKERS not set — Kafka integration disabled")
		return &Service{enabled: false}
	}

	brokers := splitBrokers(brokerList)
	log.Printf("[kafka] connecting to brokers: %v", brokers)

	return &Service{
		producer: NewProducer(brokers),
		consumer: NewConsumer(brokers, TopicReasoningRequests, DefaultGroupID),
		enabled:  true,
	}
}

// Enabled reports whether Kafka is active.
func (s *Service) Enabled() bool {
	return s.enabled
}

// PublishTrace serialises trace and publishes it to TopicReasoningTraces.
// The call is fire-and-forget: it runs in a goroutine so it never adds
// latency to the HTTP response. Errors are logged.
func (s *Service) PublishTrace(ctx context.Context, trace transformer.ReasoningTrace) {
	if !s.enabled {
		return
	}
	b, err := json.Marshal(trace)
	if err != nil {
		log.Printf("[kafka] marshal trace error: %v", err)
		return
	}
	go func() {
		if err := s.producer.Publish(ctx, TopicReasoningTraces, trace.Query, b); err != nil {
			log.Printf("[kafka] publish trace error: %v", err)
		} else {
			log.Printf("[kafka] published trace for query=%q", trace.Query)
		}
	}()
}

// PublishEvents fans out individual CoTSteps and ToolCalls to TopicCotEvents.
// Each message carries a typed key ("cot_step" or "tool_call") for downstream filtering.
// Runs in a goroutine — fire-and-forget.
func (s *Service) PublishEvents(ctx context.Context, trace transformer.ReasoningTrace) {
	if !s.enabled {
		return
	}
	go func() {
		for _, step := range trace.CoTSteps {
			b, err := json.Marshal(step)
			if err != nil {
				log.Printf("[kafka] marshal cot_step error: %v", err)
				continue
			}
			if err := s.producer.Publish(ctx, TopicCotEvents, "cot_step", b); err != nil {
				log.Printf("[kafka] publish cot_step error: %v", err)
			}
		}
		for _, tc := range trace.ToolCalls {
			b, err := json.Marshal(tc)
			if err != nil {
				log.Printf("[kafka] marshal tool_call error: %v", err)
				continue
			}
			if err := s.producer.Publish(ctx, TopicCotEvents, "tool_call", b); err != nil {
				log.Printf("[kafka] publish tool_call error: %v", err)
			}
		}
	}()
}

// StartRequestConsumer begins listening on TopicReasoningRequests.
// Each message must be a JSON object with a "query" string field.
// The pipeline is called synchronously per message (within the consumer goroutine),
// and the resulting trace is published back to TopicReasoningTraces.
// The loop exits when ctx is cancelled.
func (s *Service) StartRequestConsumer(ctx context.Context, pipeline *transformer.Pipeline) {
	if !s.enabled {
		return
	}
	s.consumer.StartLoop(ctx, func(key, value []byte) {
		var req struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(value, &req); err != nil || req.Query == "" {
			log.Printf("[kafka consumer] invalid/empty message — skipping: %s", value)
			return
		}
		log.Printf("[kafka consumer] processing async query: %q", req.Query)
		trace := pipeline.Run(req.Query)
		s.PublishTrace(ctx, trace)
		s.PublishEvents(ctx, trace)
		log.Printf("[kafka consumer] async query done: %q — %d steps", req.Query, len(trace.CoTSteps))
	})
}

// Close gracefully shuts down the producer and consumer.
// Should be called during application shutdown (e.g. after context cancellation).
func (s *Service) Close() {
	if !s.enabled {
		return
	}
	if err := s.consumer.Close(); err != nil {
		log.Printf("[kafka] consumer close error: %v", err)
	}
	if err := s.producer.Close(); err != nil {
		log.Printf("[kafka] producer close error: %v", err)
	}
	log.Println("[kafka] shutdown complete")
}

// splitBrokers parses a comma-separated broker string into a slice.
func splitBrokers(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
