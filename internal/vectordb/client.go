package vectordb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

type Client struct {
	client *weaviate.Client
}

func NewClient(host string) (*Client, error) {
	cfg := weaviate.Config{
		Host:   host,
		Scheme: "http",
	}
	client, err := weaviate.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{client: client}, nil
}

// InitSchema ensures the target class exists and is configured for our vectorizer
func (c *Client) InitSchema(ctx context.Context, className string) error {
	exists, err := c.client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if class exists: %w", err)
	}
	if exists {
		return nil
	}

	classObj := &models.Class{
		Class:       className,
		Description: "A document for hybrid search",
		Vectorizer:  "none",
		Properties: []*models.Property{
			{
				Name:     "content",
				DataType: []string{"text"},
			},
			{
				Name:     "source",
				DataType: []string{"text"},
			},
			{
				Name:     "chunk_index",
				DataType: []string{"int"},
			},
		},
	}

	err = c.client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to create class: %w", err)
	}

	log.Printf("Successfully created Weaviate schema for %s", className)
	return nil
}

type SearchResult struct {
	Content    string
	Source     string
	ChunkIndex int
	Score      float32
}

// Search performs a hybrid search if vector is provided, otherwise falls back to pure BM25
func (c *Client) HybridSearch(ctx context.Context, className string, query string, vector []float32, limit int) ([]SearchResult, error) {
	if c.client == nil {
		return nil, fmt.Errorf("weaviate client not initialized")
	}

	builder := c.client.GraphQL().Get().
		WithClassName(className).
		WithFields(
			graphql.Field{Name: "content"},
			graphql.Field{Name: "source"},
			graphql.Field{Name: "chunk_index"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "score"},
				},
			}).
		WithLimit(limit)

	if len(vector) > 0 {
		builder = builder.WithHybrid(c.client.GraphQL().HybridArgumentBuilder().
			WithQuery(query).
			WithVector(vector).
			WithAlpha(0.5)) // Alpha 0.5 balances BM25 and Vector search equally
	} else {
		builder = builder.WithBM25(c.client.GraphQL().Bm25ArgBuilder().
			WithQuery(query))
	}

	result, err := builder.Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("hybrid search failed: %w", err)
	}

	var results []SearchResult
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %v", result.Errors[0].Message)
	}

	// Parse results (weaviate returns nested maps for GraphQL queries)
	data, ok := result.Data["Get"].(map[string]interface{})
	if !ok {
		return results, nil
	}

	classData, ok := data[className].([]interface{})
	if !ok {
		return results, nil
	}

	for _, item := range classData {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		content, _ := itemMap["content"].(string)
		source, _ := itemMap["source"].(string)
		chunkIndex := intFromInterface(itemMap["chunk_index"])

		var score float32
		if additional, ok := itemMap["_additional"].(map[string]interface{}); ok {
			if s, ok := additional["score"].(string); ok {
				fmt.Sscanf(s, "%f", &score)
			}
		}

		results = append(results, SearchResult{
			Content:    content,
			Source:     source,
			ChunkIndex: chunkIndex,
			Score:      score,
		})
	}

	return results, nil
}

// AddDocument is a simple helper to add a document to Weaviate for testing
func (c *Client) AddDocument(ctx context.Context, className string, content string, vector []float32) error {
	return c.AddDocumentWithMetadata(ctx, className, content, "", 0, vector)
}

// AddDocumentWithMetadata inserts one chunk and the vector supplied by the caller.
func (c *Client) AddDocumentWithMetadata(ctx context.Context, className string, content string, source string, chunkIndex int, vector []float32) error {
	_, err := c.client.Data().Creator().
		WithClassName(className).
		WithProperties(map[string]interface{}{
			"content":     content,
			"source":      source,
			"chunk_index": chunkIndex,
		}).
		WithVector(vector).
		Do(ctx)

	if err != nil {
		log.Printf("failed to add document: %v", err)
	}
	return err
}

func intFromInterface(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}
