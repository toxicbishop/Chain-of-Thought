package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/cors"

	"cot-backend/internal/api"
	"cot-backend/internal/cache"
	"cot-backend/internal/kafka"
	"cot-backend/internal/transformer"
	"cot-backend/internal/vectordb"
)

func main() {
	// ── Load Environment ───────────────────────────────────────────────────────
	if err := godotenv.Load(".env.local"); err != nil {
		log.Printf("[main] info: .env.local not found, using system environment variables")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}


	// ── Context for graceful shutdown ──────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Transformer model ──────────────────────────────────────────────────────
	cfg := transformer.DefaultConfig()
	model := transformer.NewModel(cfg)
	pipeline := transformer.NewPipeline(model)

	// ── Redis cache ────────────────────────────────────────────────────────────
	// Reads REDIS_URL (e.g. "redis://localhost:6379").
	// Disabled gracefully when env var is absent or Redis is unreachable.
	cacheSvc := cache.NewService(os.Getenv("REDIS_URL"))
	defer cacheSvc.Close()

	// ── Kafka service ──────────────────────────────────────────────────────────
	// Reads KAFKA_BROKERS (e.g. "localhost:9092,localhost:9093").
	// Disabled gracefully when env var is absent.
	kafkaSvc := kafka.NewService(os.Getenv("KAFKA_BROKERS"))
	defer kafkaSvc.Close()

	// Start async consumer: listens on "reasoning-requests" topic.
	kafkaSvc.StartRequestConsumer(ctx, pipeline)

	// ── Vector DB ──────────────────────────────────────────────────────────────
	weaviateURL := os.Getenv("WEAVIATE_URL")
	if weaviateURL == "" {
		weaviateURL = "localhost:8081" // fallback for local dev
	}
	vdbClient, err := vectordb.NewClient(weaviateURL)
	if err != nil {
		log.Printf("[main] info: Failed to connect to Weaviate: %v", err)
	} else {
		log.Printf("[main] info: Connected to Weaviate at %s", weaviateURL)
	}

	// ── HTTP router ────────────────────────────────────────────────────────────
	router := api.NewRouter(model, kafkaSvc, cacheSvc, vdbClient)

	allowedOrigins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	if len(allowedOrigins) == 1 && allowedOrigins[0] == "" {
		allowedOrigins = []string{"http://localhost:3000"}
	}

	handler := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		ExposedHeaders: []string{"X-Cache"},
	}).Handler(router)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// ── Start server ───────────────────────────────────────────────────────────
	go func() {
		log.Printf("CoT Visualization backend listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// ── Graceful shutdown on SIGINT / SIGTERM ──────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutdown signal received — draining connections…")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	log.Println("server stopped cleanly")
}
