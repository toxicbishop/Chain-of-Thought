package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gorilla/mux"

	"cot-backend/internal/auth"
	"cot-backend/internal/cache"
	"cot-backend/internal/kafka"
	"cot-backend/internal/llm"
	"cot-backend/internal/metrics"
	"cot-backend/internal/orchestrator"
	"cot-backend/internal/transformer"
	"cot-backend/internal/vectordb"
)

const (
	// maxBodyBytes caps the size of any incoming request body (64 KB).
	maxBodyBytes = 64 * 1024
	// maxQueryLen caps the character length of the user-supplied query.
	maxQueryLen = 4096
	// pipelineTimeout is the maximum duration for a single pipeline run.
	pipelineTimeout = 120 * time.Second
)

// Router holds application-level dependencies shared across HTTP handlers.
type Router struct {
	pipeline *transformer.Pipeline
	orch     *orchestrator.Orchestrator
	kafka    *kafka.Service
	cache    *cache.Service
}

// NewRouter wires all HTTP routes and returns a ready mux.Router.
//
// Route layout:
//
//	Public  (no auth):
//	  GET  /health
//	  POST /auth/login
//
//	Protected (Bearer JWT required):
//	  GET  /auth/me
//	  POST /api/reason              ← cache-enabled
//	  POST /api/reason/stream       ← cache-aware SSE stream
//	  POST /api/attention/{layer}/{head}
//	  POST /api/activations
//	  GET  /api/kafka/status
//	  GET  /api/cache/status
//	  DELETE /api/cache             ← invalidate a query's cached trace
func NewRouter(model *transformer.Model, kafkaSvc *kafka.Service, cacheSvc *cache.Service, vdb *vectordb.Client) *mux.Router {
	r := &Router{
		pipeline: transformer.NewPipeline(model),
		orch:     orchestrator.New(vdb),
		kafka:    kafkaSvc,
		cache:    cacheSvc,
	}
	mx := mux.NewRouter()
	mx.Use(recoverPanic)

	// ── Public routes ────────────────────────────────────────────────────────
	mx.HandleFunc("/health", r.health).Methods("GET")
	mx.HandleFunc("/metrics", metrics.Handler()).Methods("GET")

	// ── Protected subrouter — all routes require a valid Bearer JWT ──────────
	protected := mx.NewRoute().Subrouter()
	protected.Use(limitBody)
	protected.Use(auth.Middleware)
	protected.Use(perUserRateLimit(newRateLimiterStore()))

	protected.HandleFunc("/auth/me", r.me).Methods("GET")
	protected.HandleFunc("/api/reason", r.reason).Methods("POST")
	protected.HandleFunc("/api/reason/stream", r.reasonStream).Methods("POST")
	protected.HandleFunc("/api/attention/{layer}/{head}", r.attention).Methods("POST")
	protected.HandleFunc("/api/activations", r.activations).Methods("POST")
	protected.HandleFunc("/api/kafka/status", r.kafkaStatus).Methods("GET")
	protected.HandleFunc("/api/cache/status", r.cacheStatus).Methods("GET")
	protected.HandleFunc("/api/cache", r.cacheInvalidate).Methods("DELETE")

	return mx
}

// ── Middleware ──────────────────────────────────────────────────────────────

// recoverPanic is the outermost middleware. It catches any panic in the
// handler chain and returns a generic 500 JSON response so raw stack traces
// and internal details are never exposed to the client.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[PANIC] %s %s — %v\n%s", r.Method, r.URL.Path, err, debug.Stack())
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"an unexpected error occurred — please try again later"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// limitBody caps the request body size to prevent oversized payloads.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// ── Helpers ─────────────────────────────────────────────────────────────────

type requestSettings struct {
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"maxTokens"`
}

type queryPayload struct {
	Query    string          `json:"query"`
	Settings requestSettings `json:"settings"`
}

// decodeQueryWithSettings decodes a JSON body with a "query" field and optional settings.
func decodeQueryWithSettings(w http.ResponseWriter, req *http.Request) (string, requestSettings, bool) {
	var body queryPayload
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return "", requestSettings{}, false
	}
	q := strings.TrimSpace(body.Query)
	if q == "" {
		http.Error(w, `{"error":"query required"}`, http.StatusBadRequest)
		return "", requestSettings{}, false
	}
	if utf8.RuneCountInString(q) > maxQueryLen {
		http.Error(w, fmt.Sprintf(`{"error":"query too long (max %d characters)"}`, maxQueryLen), http.StatusBadRequest)
		return "", requestSettings{}, false
	}
	q = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || !isControl(r) {
			return r
		}
		return -1
	}, q)
	return q, body.Settings, true
}

// decodeQuery decodes a JSON body with a "query" field. Returns the sanitised query.
func decodeQuery(w http.ResponseWriter, req *http.Request) (string, bool) {
	q, _, ok := decodeQueryWithSettings(w, req)
	return q, ok
}

func isControl(r rune) bool {
	return (r >= 0 && r <= 0x1F && r != '\n' && r != '\t') || r == 0x7F
}

// runPipeline executes the transformer pipeline with a timeout.
func (r *Router) runPipeline(parent context.Context, query string) transformer.ReasoningTrace {
	ctx, cancel := context.WithTimeout(parent, pipelineTimeout)
	defer cancel()
	return r.pipeline.RunWithContext(ctx, query)
}

// runOrchestrated executes the multi-agent orchestrator and folds the
// transformer pipeline's tokens/attentions/activations into the resulting
// trace so attention and activation endpoints keep working unchanged.
//
// events may be nil (batch mode). When non-nil, orchestrator events are
// forwarded on it and the channel is closed once both engines finish.
func (r *Router) runOrchestrated(parent context.Context, query string, events chan<- orchestrator.Event) transformer.ReasoningTrace {
	ctx, cancel := context.WithTimeout(parent, pipelineTimeout)
	defer cancel()

	// Transformer runs in parallel purely for the token/attention/activation
	// snapshots shown in the visualizer. Its own CoT steps are discarded —
	// reasoning belongs to the orchestrator now.
	pipeCh := make(chan transformer.ReasoningTrace, 1)
	go func() { pipeCh <- r.pipeline.RunWithContext(ctx, query) }()

	trace := r.orch.Run(ctx, query, events)

	p := <-pipeCh
	trace.Tokens = p.Tokens
	trace.Attentions = p.Attentions
	trace.Activations = p.Activations
	trace.ToolCalls = append(trace.ToolCalls, p.ToolCalls...)
	return trace
}

// ── Handlers ────────────────────────────────────────────────────────────────

func (r *Router) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": "1.0.0"})
}

// POST /api/reason
// Body: {"query": "..."}
//
// Flow:
//  1. Cache HIT  → return cached JSON (X-Cache: HIT)
//  2. Cache MISS → run orchestrator (with parallel transformer pass for
//                  tokens/attention/activation) → cache → return (X-Cache: MISS)
func (r *Router) reason(w http.ResponseWriter, req *http.Request) {
	query, settings, ok := decodeQueryWithSettings(w, req)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// ── Cache lookup ──────────────────────────────────────────────────────────
	if trace, ok := r.cache.GetTrace(req.Context(), query); ok {
		w.Header().Set("X-Cache", "HIT")
		json.NewEncoder(w).Encode(trace)
		return
	}
	w.Header().Set("X-Cache", "MISS")

	ctx := llm.WithSettings(req.Context(), settings.Temperature, settings.MaxTokens)

	// ── Orchestrated run ─────────────────────────────────────────────────────
	trace := r.runOrchestrated(ctx, query, nil)

	go r.cache.SetTrace(ctx, query, trace)
	r.kafka.PublishTrace(ctx, trace)
	r.kafka.PublishEvents(ctx, trace)

	json.NewEncoder(w).Encode(trace)
}

// POST /api/reason/stream
// Streams SSE events from the orchestrator live. On cache hit, replays the
// stored trace instead of re-running the pipeline.
//
// Event types:
//   meta, plan, agent_start, agent_thought, agent_done, delegation,
//   cot_step, tool_call, attention, activation, done
func (r *Router) reasonStream(w http.ResponseWriter, req *http.Request) {
	query, settings, ok := decodeQueryWithSettings(w, req)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	emit := func(event string, payload any) {
		data, _ := json.Marshal(payload)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	// ── Cache hit: replay cached trace as a stream ───────────────────────────
	if trace, hit := r.cache.GetTrace(req.Context(), query); hit {
		r.replayTrace(trace, true, emit)
		return
	}

	// ── Live run: orchestrator events forwarded as SSE ───────────────────────
	emit("meta", map[string]bool{"cache_hit": false})

	reqCtx := llm.WithSettings(req.Context(), settings.Temperature, settings.MaxTokens)
	ctx, cancel := context.WithTimeout(reqCtx, pipelineTimeout)
	defer cancel()

	events := make(chan orchestrator.Event, 32)

	// Transformer runs in parallel for tokens/attention/activation.
	pipeCh := make(chan transformer.ReasoningTrace, 1)
	go func() { pipeCh <- r.pipeline.RunWithContext(ctx, query) }()

	// Orchestrator runs in its own goroutine so the main loop can forward events.
	traceCh := make(chan transformer.ReasoningTrace, 1)
	go func() { traceCh <- r.orch.Run(ctx, query, events) }()

	for ev := range events {
		emit(ev.Type, ev.Payload)
	}

	trace := <-traceCh
	pipeTrace := <-pipeCh
	trace.Tokens = pipeTrace.Tokens
	trace.Attentions = pipeTrace.Attentions
	trace.Activations = pipeTrace.Activations
	trace.ToolCalls = append(trace.ToolCalls, pipeTrace.ToolCalls...)

	// Stream the transformer-only events after agent output so the UI can
	// populate attention/activation panels.
	for _, tc := range trace.ToolCalls {
		emit("tool_call", tc)
	}
	for _, snap := range trace.Attentions {
		if snap.Layer == 0 {
			emit("attention", snap)
		}
	}
	for _, act := range trace.Activations {
		emit("activation", act)
	}

	go r.cache.SetTrace(req.Context(), query, trace)
	r.kafka.PublishTrace(req.Context(), trace)
	r.kafka.PublishEvents(req.Context(), trace)
}

// replayTrace emits a cached trace as SSE events in roughly the same order
// the orchestrator would have streamed them live.
func (r *Router) replayTrace(trace transformer.ReasoningTrace, cached bool, emit func(string, any)) {
	emit("meta", map[string]bool{"cache_hit": cached})
	if trace.Plan != nil {
		emit("plan", trace.Plan)
	}
	for _, run := range trace.Agents {
		emit("agent_start", map[string]any{
			"id": run.ID, "name": run.Name, "role": run.Role,
			"task": run.Task, "depends_on": run.DependsOn,
		})
		if run.Thought != "" {
			emit("agent_thought", map[string]any{
				"id": run.ID, "name": run.Name, "thought": run.Thought,
			})
		}
		emit("agent_done", run)
	}
	for _, d := range trace.Delegations {
		emit("delegation", d)
	}
	for _, step := range trace.CoTSteps {
		emit("cot_step", step)
	}
	for _, tc := range trace.ToolCalls {
		emit("tool_call", tc)
	}
	for _, snap := range trace.Attentions {
		if snap.Layer == 0 {
			emit("attention", snap)
		}
	}
	for _, act := range trace.Activations {
		emit("activation", act)
	}
	emit("done", map[string]string{"answer": trace.Answer})
}

// POST /api/attention/{layer}/{head}
func (r *Router) attention(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	layerNum, err := strconv.Atoi(vars["layer"])
	if err != nil || layerNum < 0 {
		http.Error(w, `{"error":"layer must be a non-negative integer"}`, http.StatusBadRequest)
		return
	}
	headNum, err := strconv.Atoi(vars["head"])
	if err != nil || headNum < 0 {
		http.Error(w, `{"error":"head must be a non-negative integer"}`, http.StatusBadRequest)
		return
	}

	query, ok := decodeQuery(w, req)
	if !ok {
		return
	}

	// Re-use cached trace if available.
	trace, ok := r.cache.GetTrace(req.Context(), query)
	if !ok {
		trace = r.runPipeline(req.Context(), query)
		go r.cache.SetTrace(req.Context(), query, trace)
	}

	for _, snap := range trace.Attentions {
		if snap.Layer == layerNum && snap.Head == headNum {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(snap)
			return
		}
	}
	http.Error(w, `{"error":"layer/head not found"}`, http.StatusNotFound)
}

// POST /api/activations
func (r *Router) activations(w http.ResponseWriter, req *http.Request) {
	query, ok := decodeQuery(w, req)
	if !ok {
		return
	}

	trace, ok := r.cache.GetTrace(req.Context(), query)
	if !ok {
		trace = r.runPipeline(req.Context(), query)
		go r.cache.SetTrace(req.Context(), query, trace)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"activations": trace.Activations,
		"tokens":      trace.Tokens,
	})
}

// GET /api/kafka/status
func (r *Router) kafkaStatus(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := "disabled"
	if r.kafka.Enabled() {
		status = "enabled"
	}
	json.NewEncoder(w).Encode(map[string]any{
		"kafka_enabled": r.kafka.Enabled(),
		"status":        status,
		"topics": map[string]string{
			"requests": kafka.TopicReasoningRequests,
			"traces":   kafka.TopicReasoningTraces,
			"events":   kafka.TopicCotEvents,
		},
	})
}

// GET /api/cache/status
func (r *Router) cacheStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"cache_enabled": r.cache.Enabled(),
		"status":        map[bool]string{true: "enabled", false: "disabled"}[r.cache.Enabled()],
		"key_prefix":    "noetic:trace:",
		"note":          "TTL controlled by REDIS_CACHE_TTL env var (seconds, default 3600)",
	})
}

// DELETE /api/cache
// Body: {"query":"..."} — removes the cached trace for a specific query.
func (r *Router) cacheInvalidate(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query, ok := decodeQuery(w, req)
	if !ok {
		return
	}

	if err := r.cache.Invalidate(req.Context(), query); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "invalidated",
		"query":   query,
		"message": "cached trace removed",
	})
}
