package kafka

import (
	"context"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

// Producer wraps a pool of kafka-go writers, one per topic.
// Writers are created lazily on first use and reused thereafter.
type Producer struct {
	brokers []string
	writers map[string]*kafka.Writer
}

// NewProducer creates a Producer targeting the given broker addresses.
// brokers should be in "host:port" format, e.g. []string{"localhost:9092"}.
func NewProducer(brokers []string) *Producer {
	return &Producer{
		brokers: brokers,
		writers: make(map[string]*kafka.Writer),
	}
}

// writer returns (or lazily creates) the kafka.Writer for the given topic.
func (p *Producer) writer(topic string) *kafka.Writer {
	if w, ok := p.writers[topic]; ok {
		return w
	}
	w := &kafka.Writer{
		Addr:                   kafka.TCP(p.brokers...),
		Topic:                  topic,
		Balancer:               &kafka.LeastBytes{},
		WriteTimeout:           5 * time.Second,
		ReadTimeout:            5 * time.Second,
		AllowAutoTopicCreation: true, // auto-create topic on first write
	}
	p.writers[topic] = w
	return w
}

// Publish writes a single message to topic.
// key is optional but useful for log-compaction; set to "" if not needed.
func (p *Producer) Publish(ctx context.Context, topic, key string, value []byte) error {
	msg := kafka.Message{Value: value}
	if key != "" {
		msg.Key = []byte(key)
	}
	return p.writer(topic).WriteMessages(ctx, msg)
}

// Close flushes and closes all underlying writers.
// Call this during graceful shutdown.
func (p *Producer) Close() error {
	var lastErr error
	for _, w := range p.writers {
		if err := w.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
