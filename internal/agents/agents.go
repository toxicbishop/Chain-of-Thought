// Package agents defines the Agent interface, shared types, and the base
// llmAgent implementation. Each specialized agent lives in its own file.
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"cot-backend/internal/llm"
)

// Capability constants used as routing keys by the orchestrator.
const (
	CapResearch   = "research"
	CapReason     = "reason"
	CapCritique   = "critique"
	CapSynthesize = "synthesize"
	CapTool       = "tool"
)

// Task is a unit of work the orchestrator assigns to an agent. Context carries
// outputs from upstream tasks the agent may depend on; the orchestrator passes
// them by task ID.
type Task struct {
	ID          string
	Instruction string
	Context     map[string]string
}

// DelegationRequest is an agent's request that a different capability pick up
// follow-up work. The coordinator decides whether to honour it.
type DelegationRequest struct {
	To     string `json:"to"`
	Task   string `json:"task"`
	Reason string `json:"reason"`
}

// FlexString accepts both a plain JSON string and a JSON object/array.
// When the LLM returns an object for a field we expect as string, we
// flatten it back to its JSON representation so the pipeline keeps going.
type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	// Fast path: it's a normal JSON string.
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	// Slow path: object, array, number, bool, null → keep raw JSON text.
	*f = FlexString(string(b))
	return nil
}

// Result is a structured agent output.
type Result struct {
	Thought     string              `json:"thought"`
	Output      FlexString          `json:"output"`
	Confidence  float64             `json:"confidence"`
	Delegations []DelegationRequest `json:"delegations,omitempty"`
}

// Agent is the contract every specialized worker implements.
type Agent interface {
	Name() string
	Role() string
	Capability() string
	Run(ctx context.Context, task Task) (Result, error)
}

// ── Base llmAgent ───────────────────────────────────────────────────────────

// llmAgent is the shared implementation for all Gemini-backed agents.
// Concrete agents are just llmAgent values with tailored system prompts.
type llmAgent struct {
	name       string
	role       string
	capability string
	system     string
	llm        *llm.Client
	temp       float64
}

func (a *llmAgent) Name() string       { return a.name }
func (a *llmAgent) Role() string       { return a.role }
func (a *llmAgent) Capability() string { return a.capability }

func (a *llmAgent) Run(ctx context.Context, task Task) (Result, error) {
	if !a.llm.Enabled() {
		return a.fallback(task), llm.ErrNoAPIKey
	}

	user := buildUserPrompt(task)

	var res Result
	var lastErr error

	// Retry once on transient failures (rate limits, 5xx, timeouts)
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			log.Printf("[agent:%s] retrying after error: %v", a.name, lastErr)
			time.Sleep(2 * time.Second)
		}
		res = Result{}
		lastErr = a.llm.GenerateJSON(ctx, a.system+"\n\n"+responseContract, user, &res)
		if lastErr == nil {
			break
		}
	}

	if lastErr != nil {
		log.Printf("[agent:%s] LLM call failed: %v", a.name, lastErr)
		// Degrade gracefully — callers (orchestrator) still want to render a trace.
		return Result{
			Thought:    fmt.Sprintf("agent %q could not reach the LLM: %v", a.name, lastErr),
			Output:     FlexString(string(a.fallback(task).Output)),
			Confidence: 0.3,
		}, lastErr
	}
	if res.Confidence == 0 {
		res.Confidence = 0.7
	}
	return res, nil
}

// fallback produces a deterministic placeholder when the LLM is unavailable so
// the orchestrator still emits a coherent trace in demo/offline mode.
func (a *llmAgent) fallback(task Task) Result {
	return Result{
		Thought:    fmt.Sprintf("[offline] %s considered: %s", a.role, task.Instruction),
		Output:     FlexString(fmt.Sprintf("[offline stub from %s] %s", a.name, task.Instruction)),
		Confidence: 0.4,
	}
}

// responseContract is appended to every system prompt so all agents return the
// same JSON shape. Kept short to save tokens.
const responseContract = `Reply with ONLY a JSON object, no prose, no markdown fences:
{
  "thought": "brief internal reasoning (1-3 sentences)",
  "output": "the concrete deliverable — facts, the conclusion, the critique, etc.",
  "confidence": 0.0-1.0,
  "delegations": [{"to":"<capability>","task":"<instruction>","reason":"<why>"}]
}
"delegations" is optional — omit or use [] if none.`

func buildUserPrompt(task Task) string {
	var b strings.Builder
	b.WriteString("TASK: ")
	b.WriteString(task.Instruction)
	b.WriteString("\n")

	if len(task.Context) > 0 {
		b.WriteString("\nUPSTREAM RESULTS:\n")
		for id, out := range task.Context {
			fmt.Fprintf(&b, "- [%s]: %s\n", id, truncate(out, 1200))
		}
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
