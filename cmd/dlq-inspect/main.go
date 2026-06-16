package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	brokers := flag.String("brokers", "localhost:9092", "Kafka brokers comma separated")
	topic := flag.String("topic", "reasoning-requests-dlq", "DLQ topic to inspect")
	limit := flag.Int("limit", 10, "Max messages to read (0 for no limit)")
	flag.Parse()

	log.Printf("Connecting to Kafka brokers: %s", *brokers)
	log.Printf("Inspecting topic: %s", *topic)

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{*brokers},
		Topic:       *topic,
		Partition:   0,
		MinBytes:    1,
		MaxBytes:    10e6, // 10MB
		StartOffset: kafka.FirstOffset, // Start from beginning to inspect existing DLQ messages
	})
	defer r.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Interrupt received, shutting down...")
		cancel()
	}()

	count := 0
	for {
		if *limit > 0 && count >= *limit {
			log.Printf("Reached limit of %d messages, exiting.", *limit)
			break
		}
		
		// Try to fetch next message, use a timeout to avoid blocking forever
		// if we've reached the end of the topic.
		readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
		msg, err := r.ReadMessage(readCtx)
		readCancel()
		
		if err != nil {
			if readCtx.Err() == context.DeadlineExceeded {
				log.Println("No more messages in DLQ (timeout waiting for new messages).")
				break
			}
			if ctx.Err() != nil {
				break
			}
			log.Printf("Error reading message: %v", err)
			continue
		}

		fmt.Printf("\n--- Message %d ---\n", count+1)
		fmt.Printf("Offset: %d\n", msg.Offset)
		fmt.Printf("Time:   %s\n", msg.Time.Format(time.RFC3339))
		fmt.Printf("Key:    %s\n", string(msg.Key))
		fmt.Printf("Value:  %s\n", string(msg.Value))
		count++
	}
	log.Printf("Inspected %d messages.", count)
}
