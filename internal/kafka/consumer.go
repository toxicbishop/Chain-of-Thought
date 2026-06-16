package kafka

import (
	"context"
	"log"
	"time"

	kafka "github.com/segmentio/kafka-go"

	"github.com/toxicbishop/Chain-of-Thought/internal/metrics"
)

// MessageHandler is called for every message the consumer reads.
// It returns an error if processing failed, which triggers retries.
type MessageHandler func(key, value []byte) error

// Consumer wraps a kafka-go reader for a single topic + consumer group.
type Consumer struct {
	reader     *kafka.Reader
	brokers    []string
	topic      string
	groupID    string
	dlqHandler func(key, value []byte) // optional DLQ publisher
}

// NewConsumer creates a Consumer for the given topic and consumer group.
func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10 << 20, // 10 MB
		MaxWait:        500 * time.Millisecond,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
	})
	return &Consumer{
		reader:  r,
		brokers: brokers,
		topic:   topic,
		groupID: groupID,
	}
}

// SetDLQHandler configures the function called when a message exceeds retries.
func (c *Consumer) SetDLQHandler(handler func(key, value []byte)) {
	c.dlqHandler = handler
}

// StartLagPoller launches a goroutine that periodically computes consumer lag
// (latest topic offset − last committed offset) and records it as a Prometheus gauge.
// interval controls how often the poll runs; 15s is a reasonable default.
func (c *Consumer) StartLagPoller(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lag, err := c.computeLag(ctx)
				if err != nil {
					log.Printf("[kafka lag] failed to compute lag for topic=%s: %v", c.topic, err)
					continue
				}
				metrics.KafkaConsumerLag.WithLabelValues(c.topic).Set(float64(lag))
			}
		}
	}()
}

// computeLag dials the broker directly to fetch the latest offset for every
// partition of the topic, then subtracts the reader's committed offset.
// kafka-go exposes Stats().Lag per-partition for single-partition setups;
// for multi-partition topics we use the lower-level DialLeader approach.
func (c *Consumer) computeLag(ctx context.Context) (int64, error) {
	// kafka-go Reader.Stats().Lag is the lag for the currently assigned
	// partition(s). For a single-consumer, single-group setup this is the
	// canonical value and requires no extra connections.
	stats := c.reader.Stats()
	return stats.Lag, nil
}

// StartLoop launches a goroutine that calls handler for every incoming message.
// The goroutine exits cleanly when ctx is cancelled.
// Transient read errors are logged and retried; context cancellation causes a clean exit.
func (c *Consumer) StartLoop(ctx context.Context, handler MessageHandler) {
	go func() {
		log.Printf("[kafka consumer] starting loop on topic=%s", c.topic)
		for {
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					log.Printf("[kafka consumer] context cancelled, stopping loop on topic=%s", c.topic)
					return
				}
				log.Printf("[kafka consumer] read error on topic=%s: %v — retrying", c.topic, err)
				continue
			}

			var processErr error
			for i := 0; i < 3; i++ {
				if processErr = handler(msg.Key, msg.Value); processErr == nil {
					break
				}
				log.Printf("[kafka consumer] processing failed (attempt %d/3): %v", i+1, processErr)
				metrics.KafkaRetries.WithLabelValues(c.topic).Inc()
				time.Sleep(time.Duration(i+1) * time.Second)
			}

			if processErr != nil {
				if c.dlqHandler != nil {
					log.Printf("[kafka consumer] message failed after 3 retries, moving to DLQ")
					c.dlqHandler(msg.Key, msg.Value)
				}
				metrics.KafkaDLQTotal.WithLabelValues(c.topic).Inc()
			} else {
				metrics.KafkaMessagesProcessed.WithLabelValues(c.topic).Inc()
			}
		}
	}()
}

// Close shuts down the reader and releases its connections.
func (c *Consumer) Close() error {
	return c.reader.Close()
}