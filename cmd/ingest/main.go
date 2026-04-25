package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"cot-backend/internal/llm"
	"cot-backend/internal/vectordb"
)

func main() {
	if err := godotenv.Load(".env.local"); err != nil {
		log.Printf("info: .env.local not found")
	}

	weaviateURL := os.Getenv("WEAVIATE_URL")
	if weaviateURL == "" {
		weaviateURL = "localhost:8081" // fallback for local dev
	}

	log.Printf("Connecting to Weaviate at %s...", weaviateURL)
	vdbClient, err := vectordb.NewClient(weaviateURL)
	if err != nil {
		log.Fatalf("Failed to connect to Weaviate: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize the schema
	log.Println("Initializing Schema...")
	err = vdbClient.InitSchema(ctx, "Document")
	if err != nil {
		log.Fatalf("Failed to init schema: %v", err)
	}

	// Read RAG.txt
	data, err := os.ReadFile("RAG.txt")
	if err != nil {
		log.Fatalf("Failed to read RAG.txt: %v", err)
	}

	content := string(data)
	
	// Simple chunking strategy: split by blank lines
	chunks := strings.Split(content, "\n\n")

	// Initialize Gemini Client
	llmClient := llm.New()
	if !llmClient.Enabled() {
		log.Fatalf("GEMINI_API_KEY is not set. We need it to compute embeddings.")
	}

	log.Printf("Found %d chunks. Inserting...", len(chunks))
	inserted := 0
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}

		vec, err := llmClient.Embed(ctx, chunk)
		if err != nil {
			log.Printf("Warning: Failed to embed chunk (%v), falling back to BM25-only index for this chunk", err)
			vec = nil
		}

		err = vdbClient.AddDocument(ctx, "Document", chunk, vec)
		if err != nil {
			log.Printf("Failed to insert chunk: %v", err)
		} else {
			inserted++
		}
	}

	log.Printf("Successfully inserted %d documents into Weaviate!", inserted)
}
