// Package orchestrator – planner.go
// The Planner decomposes a user query into a DAG of sub-tasks that the
// Executor will run. It tries the LLM first and falls back to a static
// 4-step DAG when the model is unavailable or returns an invalid plan.
package orchestrator

import (
	"context"

	"cot-backend/internal/agents"
	"cot-backend/internal/llm"
	"cot-backend/internal/transformer"
)

// Planner turns a natural-language query into a validated TaskPlan DAG.
type Planner struct {
	llm *llm.Client
}

// NewPlanner creates a Planner backed by the given LLM client.
func NewPlanner(client *llm.Client) *Planner {
	return &Planner{llm: client}
}

// ── Prompts ─────────────────────────────────────────────────────────────────

const plannerSystem = `You are a task planner for a multi-agent reasoning system.
Decompose the user's question into a small DAG of sub-tasks.

Available capabilities: research, reason, critique, synthesize, tool

Rules:
- Use 3 to 5 tasks total.
- Task ids are simple strings: t1, t2, t3, ...
- Every task's depends_on must reference only earlier task ids (strict DAG).
- The LAST task must have capability "synthesize" and depend on the others.
- Pick the smallest plan that genuinely answers the question — do not pad.`

const plannerContract = `Reply with ONLY a JSON object, no prose, no fences:
{
  "goal": "restatement of the user's goal",
  "tasks": [
    {"id":"t1","agent":"research","task":"...","depends_on":[]},
    {"id":"t2","agent":"reason","task":"...","depends_on":["t1"]}
  ]
}`

// Plan generates a TaskPlan for the given query.
func (p *Planner) Plan(ctx context.Context, query string) transformer.TaskPlan {
	if !p.llm.Enabled() {
		return DefaultPlan(query)
	}
	var plan transformer.TaskPlan
	user := "USER QUESTION: " + query
	if err := p.llm.GenerateJSON(ctx, plannerSystem+"\n\n"+plannerContract, user, &plan); err != nil {
		return DefaultPlan(query)
	}
	if !ValidatePlan(plan) {
		return DefaultPlan(query)
	}
	return plan
}

// DefaultPlan is used when the LLM is unavailable or produces an invalid plan.
// It is a canonical 4-step research → reason → critique → synthesize DAG.
func DefaultPlan(query string) transformer.TaskPlan {
	return transformer.TaskPlan{
		Goal: query,
		Tasks: []transformer.PlannedTask{
			{ID: "t1", Agent: agents.CapResearch, Task: "Gather the key facts, definitions, and sub-questions relevant to: " + query},
			{ID: "t2", Agent: agents.CapReason, Task: "Produce a step-by-step argument that answers: " + query, DependsOn: []string{"t1"}},
			{ID: "t3", Agent: agents.CapCritique, Task: "Audit the reasoning above for errors, gaps, and weak steps.", DependsOn: []string{"t2"}},
			{ID: "t4", Agent: agents.CapSynthesize, Task: "Produce the final answer integrating research, reasoning, and critique.", DependsOn: []string{"t1", "t2", "t3"}},
		},
	}
}

// ValidatePlan checks structural integrity of a generated plan.
func ValidatePlan(p transformer.TaskPlan) bool {
	if len(p.Tasks) < 2 || len(p.Tasks) > 8 {
		return false
	}
	seen := map[string]bool{}
	for _, t := range p.Tasks {
		if t.ID == "" || t.Agent == "" || t.Task == "" {
			return false
		}
		for _, dep := range t.DependsOn {
			if !seen[dep] {
				return false
			}
		}
		seen[t.ID] = true
	}
	last := p.Tasks[len(p.Tasks)-1]
	return last.Agent == agents.CapSynthesize
}
