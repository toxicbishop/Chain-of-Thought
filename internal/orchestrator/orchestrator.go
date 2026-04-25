// Package orchestrator coordinates multiple specialized agents to answer a
// single user query. It delegates planning to the Planner and execution to
// the Executor, keeping its own surface minimal and focused on DAG traversal
// and delegation handling.
package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"cot-backend/internal/agents"
	"cot-backend/internal/llm"
	"cot-backend/internal/transformer"
	"cot-backend/internal/vectordb"
)

const (
	// maxDelegationsPerAgent caps how many sibling delegations a single agent
	// can spawn, to keep runtime bounded.
	maxDelegationsPerAgent = 2
)

// Event is one item on the orchestrator's event stream. Type is one of:
// "plan", "agent_start", "agent_thought", "agent_done", "delegation",
// "cot_step", "done".
type Event struct {
	Type    string
	Payload any
}

// Orchestrator is the public entry point. It owns the agent roster,
// a Planner for decomposition, and an Executor for dispatch.
type Orchestrator struct {
	planner  *Planner
	executor *Executor
	llm      *llm.Client
	agents   map[string]agents.Agent // capability → agent
}

// New builds an Orchestrator with the default agent roster.
func New(vdb *vectordb.Client) *Orchestrator {
	return NewWithClient(llm.New(), vdb)
}

// NewWithClient lets callers inject a preconfigured LLM client (useful for tests).
func NewWithClient(client *llm.Client, vdb *vectordb.Client) *Orchestrator {
	roster := agents.Default(client, vdb)
	m := make(map[string]agents.Agent, len(roster))
	for _, a := range roster {
		m[a.Capability()] = a
	}
	return &Orchestrator{
		planner:  NewPlanner(client),
		executor: NewExecutor(),
		llm:      client,
		agents:   m,
	}
}

// Enabled reports whether the orchestrator has a live LLM connection.
func (o *Orchestrator) Enabled() bool { return o.llm.Enabled() }

// Run executes the full orchestration pipeline and returns the populated trace.
// The events channel is closed when execution finishes. Pass a nil channel to
// discard events.
func (o *Orchestrator) Run(ctx context.Context, query string, events chan<- Event) transformer.ReasoningTrace {
	defer func() {
		if events != nil {
			close(events)
		}
	}()

	trace := transformer.ReasoningTrace{
		Query:  query,
		Engine: "orchestrator",
	}

	// ── 1. Plan ──────────────────────────────────────────────────────────────
	plan := o.planner.Plan(ctx, query)
	trace.Plan = &plan
	emit(events, Event{Type: "plan", Payload: plan})

	// ── 2. Execute DAG with parallel resolution ──────────────────────────────
	state := NewExecutionState(DefaultGuardrails())
	results := o.ExecuteDAG(ctx, plan, state, events)

	stepIdx := 0
	for _, dr := range results {
		trace.Agents = append(trace.Agents, dr.Run)

		if step, ok := CotStepFrom(dr.Run, stepIdx); ok {
			trace.CoTSteps = append(trace.CoTSteps, step)
			emit(events, Event{Type: "cot_step", Payload: step})
			stepIdx++
		}

		// ── 3. Honour delegation requests (bounded by guardrails) ─────────────
		for i, req := range dr.Res.Delegations {
			if i >= maxDelegationsPerAgent {
				break
			}
			if err := state.RecordAgentRun(); err != nil {
				break // guardrail tripped
			}

			target := o.route(req.To)
			if target == nil || target.Name() == dr.Run.Name {
				continue
			}

			// Check critic loop guardrail.
			if req.To == agents.CapResearch && dr.Run.Role == "verifier" {
				if err := state.RecordCriticLoop(); err != nil {
					break
				}
			}

			delegID := fmt.Sprintf("%s.d%d", dr.TaskID, i+1)
			record := transformer.Delegation{
				From:   dr.Run.Name,
				To:     target.Name(),
				Reason: req.Reason,
				TaskID: delegID,
			}
			trace.Delegations = append(trace.Delegations, record)
			emit(events, Event{Type: "delegation", Payload: record})

			delegTask := agents.Task{
				ID:          delegID,
				Instruction: req.Task,
				Context:     state.GatherContext([]string{dr.TaskID}),
			}
			delegRun, _ := o.executor.RunAgent(ctx, target, delegTask, []string{dr.TaskID}, events)
			trace.Agents = append(trace.Agents, delegRun)
			state.SetOutput(delegID, delegRun.Output)

			if step, ok := CotStepFrom(delegRun, stepIdx); ok {
				trace.CoTSteps = append(trace.CoTSteps, step)
				emit(events, Event{Type: "cot_step", Payload: step})
				stepIdx++
			}
		}
	}

	// ── 4. Final answer = last Synthesizer output ───────────────────────────
	trace.Answer = FinalAnswer(trace.Agents, query)
	emit(events, Event{Type: "done", Payload: map[string]string{"answer": trace.Answer}})
	return trace
}

// route maps a capability label to the concrete agent that serves it, with
// a Reasoner fallback for unknown labels.
func (o *Orchestrator) route(capability string) agents.Agent {
	if a, ok := o.agents[capability]; ok {
		return a
	}
	return o.agents[agents.CapReason]
}

// ── Shared Helpers ──────────────────────────────────────────────────────────

func gatherContext(depIDs []string, outputs map[string]string) map[string]string {
	if len(depIDs) == 0 {
		return nil
	}
	ctx := make(map[string]string, len(depIDs))
	for _, id := range depIDs {
		if v, ok := outputs[id]; ok {
			ctx[id] = v
		}
	}
	return ctx
}

func firstLine(s string) string {
	head, _, _ := strings.Cut(s, "\n")
	return strings.TrimSpace(head)
}

func emit(ch chan<- Event, ev Event) {
	if ch == nil {
		return
	}
	// Guard against send-on-closed-channel when parallel DAG goroutines
	// race against the deferred close in Run().
	defer func() { recover() }()
	ch <- ev
}
