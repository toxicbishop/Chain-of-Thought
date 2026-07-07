package kafka_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	segmentiokafka "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	testcontainerskafka "github.com/testcontainers/testcontainers-go/modules/kafka"

	"cot-backend/internal/kafka"
)

func TestDLQPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()

	// 1. Start Kafka container
	kafkaContainer, err := testcontainerskafka.Run(ctx,
		"confluentinc/confluent-local:7.6.0",
	)
	require.NoError(t, err)
	defer func() {
		if err := kafkaContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	brokers, err := kafkaContainer.Brokers(ctx)
	require.NoError(t, err)

	broker := brokers[0]
	
	// Create producer
	producer := kafka.NewProducer([]string{broker})
	defer producer.Close()

	dlqTopic := "test-dlq"
	requestsTopic := "test-requests"

	// 2. Set up Consumer with DLQ handler
	consumer := kafka.NewConsumer([]string{broker}, requestsTopic, "test-group")
	defer consumer.Close()
	
	consumer.SetDLQHandler(func(key, value []byte) {
		producer.Publish(ctx, dlqTopic, string(key), value)
	})

	// 3. Start a failing consumer loop
	consumerCtx, cancelConsumer := context.WithCancel(ctx)
	defer cancelConsumer()

	var attemptCount int
	consumer.StartLoop(consumerCtx, func(key, value []byte) error {
		attemptCount++
		// always fail
		return fmt.Errorf("forced failure for test")
	})

	// Wait a moment for consumer group to stabilize
	time.Sleep(2 * time.Second)

	// 4. Produce a test message to the main topic
	err = producer.Publish(ctx, requestsTopic, "test-key", []byte(`{"query":"test message"}`))
	require.NoError(t, err)

	// Wait enough time for 3 retries (1s + 2s + 3s = 6 seconds, plus buffer)
	// Actually StartLoop has a delay of (i+1) seconds. 1 + 2 + 3 = 6 seconds.
	time.Sleep(10 * time.Second)

	// 5. Assert the message landed in the DLQ topic
	r := segmentiokafka.NewReader(segmentiokafka.ReaderConfig{
		Brokers:   []string{broker},
		Topic:     dlqTopic,
		Partition: 0,
		MaxWait:   time.Second,
	})
	defer r.Close()

	readCtx, cancelRead := context.WithTimeout(ctx, 10*time.Second)
	defer cancelRead()

	msg, err := r.ReadMessage(readCtx)
	require.NoError(t, err, "should read a message from DLQ")
	
	require.Equal(t, "test-key", string(msg.Key))
	require.Equal(t, `{"query":"test message"}`, string(msg.Value))
}
