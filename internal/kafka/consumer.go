package kafka

import (
	"context"
	"log"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

// MessageHandler is called for every message the consumer reads.
// It returns an error if processing failed, which triggers retries.
type MessageHandler func(key, value []byte) error

// Consumer wraps a kafka-go reader for a single topic + consumer group.
type Consumer struct {
	reader     *kafka.Reader
	topic      string
	dlqHandler func(key, value []byte) // optional DLQ publisher
}

// NewConsumer creates a Consumer for the given topic and consumer group.
func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,        // fetch as soon as a byte is available
		MaxBytes:       10 << 20, // 10 MB
		MaxWait:        500 * time.Millisecond,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset, // start from new messages only
	})
	return &Consumer{reader: r, topic: topic}
}

// SetDLQHandler configures the function called when a message exceeds retries.
func (c *Consumer) SetDLQHandler(handler func(key, value []byte)) {
	c.dlqHandler = handler
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
				time.Sleep(time.Duration(i+1) * time.Second)
			}
			
			if processErr != nil && c.dlqHandler != nil {
				log.Printf("[kafka consumer] message failed after 3 retries, moving to DLQ")
				c.dlqHandler(msg.Key, msg.Value)
			}
		}
	}()
}

// Close shuts down the reader and releases its connections.
func (c *Consumer) Close() error {
	return c.reader.Close()
}
