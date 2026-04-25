package kafka

// Topic names used across the service.
const (
	// TopicReasoningRequests is consumed by the async worker: each message
	// contains a JSON {"query":"..."} and triggers a pipeline run.
	TopicReasoningRequests = "reasoning-requests"

	// TopicReasoningTraces receives the full ReasoningTrace JSON produced after
	// every pipeline run (both HTTP-triggered and consumer-triggered).
	TopicReasoningTraces = "reasoning-traces"

	// TopicCotEvents receives one message per CoTStep and one per ToolCall,
	// enabling stream-processing consumers to react to individual reasoning events.
	TopicCotEvents = "cot-events"

	// DefaultGroupID is the consumer group used by the built-in request consumer.
	DefaultGroupID = "noetic-consumer-group"
)
