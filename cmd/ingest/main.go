package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/joho/godotenv"

	"cot-backend/internal/llm"
	"cot-backend/internal/vectordb"
)

func main() {
	if err := godotenv.Load(".env.local"); err != nil {
		log.Printf("info: .env.local not found")
	}

	sourcePath := flag.String("source", "RAG.txt", "path to a text file to ingest")
	inlineText := flag.String("text", "", "inline text to ingest instead of -source")
	className := flag.String("class", "Document", "Weaviate class name")
	chunkSize := flag.Int("chunk-size", 800, "target chunk size in characters")
	overlap := flag.Int("overlap", 80, "character overlap between long chunks")
	timeout := flag.Duration("timeout", 2*time.Minute, "overall ingestion timeout")
	flag.Parse()

	content, source := loadContent(*sourcePath, *inlineText)
	chunks := chunkText(content, *chunkSize, *overlap)
	if len(chunks) == 0 {
		log.Fatalf("no ingestible text found in %s", source)
	}

	weaviateURL := envOr("WEAVIATE_URL", "localhost:8081")
	log.Printf("Connecting to Weaviate at %s...", weaviateURL)
	vdbClient, err := vectordb.NewClient(weaviateURL)
	if err != nil {
		log.Fatalf("Failed to connect to Weaviate: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	log.Printf("Initializing schema %q...", *className)
	if err := vdbClient.InitSchema(ctx, *className); err != nil {
		log.Fatalf("Failed to init schema: %v", err)
	}

	llmClient := llm.New()
	if !llmClient.Enabled() {
		log.Fatalf("GEMINI_API_KEY is not set. We need it to compute embeddings.")
	}

	log.Printf("Embedding %d chunks from %s with %s...", len(chunks), source, llmClient.EmbeddingModel())
	inserted := 0
	for i, chunk := range chunks {
		vec, err := llmClient.Embed(ctx, chunk)
		if err != nil {
			log.Fatalf("failed to embed chunk %d: %v", i, err)
		}

		if err := vdbClient.AddDocumentWithMetadata(ctx, *className, chunk, source, i, vec); err != nil {
			log.Printf("Failed to insert chunk %d: %v", i, err)
			continue
		}
		inserted++
	}

	log.Printf("Successfully inserted %d/%d chunks into Weaviate class %q.", inserted, len(chunks), *className)
}

func loadContent(sourcePath, inlineText string) (string, string) {
	if strings.TrimSpace(inlineText) != "" {
		return inlineText, "inline"
	}

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", sourcePath, err)
	}
	return string(data), filepath.Base(sourcePath)
}

func chunkText(text string, chunkSize, overlap int) []string {
	text = normalizeWhitespace(text)
	if text == "" {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = 800
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 5
	}

	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var current strings.Builder

	flush := func() {
		chunk := strings.TrimSpace(current.String())
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		current.Reset()
	}

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		if utf8.RuneCountInString(paragraph) > chunkSize {
			flush()
			chunks = append(chunks, splitLongText(paragraph, chunkSize, overlap)...)
			continue
		}
		nextLen := utf8.RuneCountInString(current.String()) + utf8.RuneCountInString(paragraph) + 2
		if current.Len() > 0 && nextLen > chunkSize {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(paragraph)
	}
	flush()

	return chunks
}

func splitLongText(text string, chunkSize, overlap int) []string {
	runes := []rune(text)
	step := chunkSize - overlap
	if step <= 0 {
		step = chunkSize
	}

	var chunks []string
	for start := 0; start < len(runes); start += step {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}

func normalizeWhitespace(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
