// Package llm provides a minimal Gemini REST client used by the orchestrator
// and its specialized agents. Reading GEMINI_API_KEY is the only configuration;
// when the key is absent the client returns a sentinel error so callers can
// fall back to deterministic output instead of failing hard.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ErrNoAPIKey is returned when GEMINI_API_KEY is not configured.
var ErrNoAPIKey = errors.New("llm: GEMINI_API_KEY is not set")

const (
	defaultModel          = "gemini-2.0-flash"
	defaultEmbeddingModel = "gemini-embedding-001"
	defaultEndpoint       = "https://generativelanguage.googleapis.com/v1beta/models"
	defaultTimeout        = 60 * time.Second
)

// Client is a minimal Gemini REST client. The zero value is unusable — use New.
type Client struct {
	apiKey   string
	embedKey string
	model    string
	embed    string
	endpoint string
	http     *http.Client
	isOR     bool
}

// New constructs a Client reading OPEN_ROUTER.AI or GEMINI_API_KEY from the environment.
// When both keys are absent the returned Client is disabled: every call returns
// ErrNoAPIKey. Enabled() can be used to detect this before calling.
func New() *Client {
	if orKey := os.Getenv("OPEN_ROUTER.AI"); orKey != "" {
		return &Client{
			apiKey:   orKey,
			embedKey: os.Getenv("GEMINI_API_KEY"),
			model:    envOr("OPENROUTER_MODEL", "google/gemini-2.5-flash"), // OpenRouter's recommended default for Gemini
			embed:    envOr("GEMINI_EMBEDDING_MODEL", defaultEmbeddingModel),
			endpoint: "https://openrouter.ai/api/v1/chat/completions",
			http:     &http.Client{Timeout: defaultTimeout},
			isOR:     true,
		}
	}

	return &Client{
		apiKey:   os.Getenv("GEMINI_API_KEY"),
		embedKey: os.Getenv("GEMINI_API_KEY"),
		model:    envOr("GEMINI_MODEL", defaultModel),
		embed:    envOr("GEMINI_EMBEDDING_MODEL", defaultEmbeddingModel),
		endpoint: defaultEndpoint,
		http:     &http.Client{Timeout: defaultTimeout},
		isOR:     false,
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// Enabled reports whether the client is configured with an API key.
func (c *Client) Enabled() bool { return c != nil && c.apiKey != "" }

// Model returns the configured model identifier.
func (c *Client) Model() string { return c.model }

// EmbeddingModel returns the configured Gemini embedding model identifier.
func (c *Client) EmbeddingModel() string { return c.embed }

// GenOpts tunes a single generation call.
type GenOpts struct {
	Temperature float64
	MaxTokens   int
	JSON        bool // request application/json output
}

// Generate runs a single non-streaming generation and returns the text.
// When opts.JSON is true the response is guaranteed to be a JSON document
// (the caller still owns parsing).
func (c *Client) Generate(ctx context.Context, system, user string, opts GenOpts) (string, error) {
	if !c.Enabled() {
		return "", ErrNoAPIKey
	}

	if c.isOR {
		return c.generateOpenRouter(ctx, system, user, opts)
	}

	body := geminiRequest{
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: user}}},
		},
		GenerationConfig: geminiGenConfig{
			Temperature: opts.Temperature,
		},
	}
	if system != "" {
		body.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: system}}}
	}
	if opts.MaxTokens > 0 {
		body.GenerationConfig.MaxOutputTokens = opts.MaxTokens
	}
	if opts.JSON {
		body.GenerationConfig.ResponseMimeType = "application/json"
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("llm: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", c.endpoint, c.model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("llm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm: status %d: %s", resp.StatusCode, truncate(string(respBody), 400))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("llm: empty response")
	}

	var out strings.Builder
	for _, p := range parsed.Candidates[0].Content.Parts {
		out.WriteString(p.Text)
	}
	return out.String(), nil
}

type settingsKey struct{}

// WithSettings attaches dynamic inference parameters to the context.
func WithSettings(ctx context.Context, temp float64, maxTokens int) context.Context {
	return context.WithValue(ctx, settingsKey{}, GenOpts{Temperature: temp, MaxTokens: maxTokens})
}

// GenerateJSON runs a generation in JSON mode and unmarshals the response.
// it into the provided destination.
func (c *Client) GenerateJSON(ctx context.Context, system, user string, dst any) error {
	opts := GenOpts{Temperature: 0.2, JSON: true}
	if ctxOpts, ok := ctx.Value(settingsKey{}).(GenOpts); ok {
		if ctxOpts.Temperature >= 0 {
			opts.Temperature = ctxOpts.Temperature
		}
		if ctxOpts.MaxTokens > 0 {
			opts.MaxTokens = ctxOpts.MaxTokens
		}
	}

	text, err := c.Generate(ctx, system, user, opts)
	if err != nil {
		return err
	}
	// Gemini sometimes wraps JSON in ```json fences even in JSON mode — strip.
	text = stripJSONFences(text)
	return json.Unmarshal([]byte(text), dst)
}

func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ── OpenRouter implementation ───────────────────────────────────────────────

type orMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type orRequest struct {
	Model       string      `json:"model"`
	Messages    []orMessage `json:"messages"`
	Temperature float64     `json:"temperature,omitempty"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
}

type orResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) generateOpenRouter(ctx context.Context, system, user string, opts GenOpts) (string, error) {
	reqBody := orRequest{
		Model: c.model,
		Messages: []orMessage{
			{Role: "user", Content: user},
		},
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
	}
	if system != "" {
		reqBody.Messages = append([]orMessage{{Role: "system", Content: system}}, reqBody.Messages...)
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("llm: marshal OR request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("llm: build OR request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("HTTP-Referer", "http://localhost:3000")
	req.Header.Set("X-Title", "LogicFlow")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: http OR: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm: OR status %d: %s", resp.StatusCode, truncate(string(respBody), 400))
	}

	var parsed orResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("llm: decode OR response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("llm: empty OR response")
	}

	return parsed.Choices[0].Message.Content, nil
}

// ── Embeddings ──────────────────────────────────────────────────────────────

type geminiEmbedRequest struct {
	Model   string        `json:"model"`
	Content geminiContent `json:"content"`
}

type geminiEmbedResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

// Embed computes a vector embedding for the given text using Gemini.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	if c == nil || c.embedKey == "" {
		return nil, ErrNoAPIKey
	}

	model := strings.TrimPrefix(c.embed, "models/")
	body := geminiEmbedRequest{
		Model: "models/" + model,
		Content: geminiContent{
			Parts: []geminiPart{{Text: text}},
		},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("llm: marshal embed request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:embedContent?key=%s", defaultEndpoint, model, c.embedKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("llm: build embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: http embed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm: embed status %d: %s", resp.StatusCode, truncate(string(respBody), 400))
	}

	var parsed geminiEmbedResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("llm: decode embed response: %w", err)
	}
	if len(parsed.Embedding.Values) == 0 {
		return nil, fmt.Errorf("llm: empty embedding")
	}

	return parsed.Embedding.Values, nil
}

// ── Wire types ──────────────────────────────────────────────────────────────

type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	Temperature      float64 `json:"temperature,omitempty"`
	MaxOutputTokens  int     `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
}
