// Package orchestrator – executor.go
// The Executor runs individual agents, records their AgentRun traces,
// extracts CoT steps and tool calls, and resolves the final answer.
package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cot-backend/internal/agents"
	"cot-backend/internal/metrics"
	"cot-backend/internal/transformer"
)

// Executor handles the low-level agent dispatch and trace recording.
type Executor struct{}

// NewExecutor creates an Executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// RunAgent runs a single agent and returns both the recorded AgentRun (for the
// trace) and the raw Result (so the caller can act on delegations).
func (e *Executor) RunAgent(
	ctx context.Context,
	a agents.Agent,
	task agents.Task,
	dependsOn []string,
	events chan<- Event,
) (transformer.AgentRun, agents.Result) {
	start := time.Now()

	emit(events, Event{Type: "agent_start", Payload: map[string]any{
		"id":         task.ID,
		"name":       a.Name(),
		"role":       a.Role(),
		"task":       task.Instruction,
		"depends_on": dependsOn,
	}})

	res, err := a.Run(ctx, task)
	status := "done"
	if err != nil {
		status = "failed"
	}

	if res.Thought != "" {
		emit(events, Event{Type: "agent_thought", Payload: map[string]any{
			"id":      task.ID,
			"name":    a.Name(),
			"thought": res.Thought,
		}})
	}

	run := transformer.AgentRun{
		ID:         task.ID,
		Name:       a.Name(),
		Role:       a.Role(),
		Task:       task.Instruction,
		Thought:    res.Thought,
		Output:     string(res.Output),
		Confidence: res.Confidence,
		Status:     status,
		DependsOn:  dependsOn,
		StartedAt:  start.UTC().Format(time.RFC3339Nano),
		EndedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		ToolCalls:  extractToolCalls(a, res),
	}

	metrics.RecordAgentRun(a.Name(), status, time.Since(start))
	emit(events, Event{Type: "agent_done", Payload: run})
	return run, res
}

// extractToolCalls parses simulated tool invocations from a ToolAgent's thought.
func extractToolCalls(a agents.Agent, res agents.Result) []transformer.ToolCall {
	if a.Capability() != agents.CapTool {
		return nil
	}
	parts := strings.SplitN(res.Thought, ":", 2)
	if len(parts) < 2 {
		return nil
	}
	return []transformer.ToolCall{{
		Name:   strings.TrimSpace(parts[0]),
		Inputs: map[string]string{"input": strings.TrimSpace(parts[1])},
		Output: string(res.Output),
	}}
}

// CotStepFrom converts an AgentRun into a structured CoTStep for the trace.
func CotStepFrom(run transformer.AgentRun, idx int) (transformer.CoTStep, bool) {
	stepType := "inference"
	switch run.Role {
	case "fact gatherer":
		stepType = "premise"
	case "final answer":
		stepType = "conclusion"
	case "tool executor":
		stepType = "tool_call"
	}
	text := run.Name + ": " + firstLine(run.Output)
	if strings.TrimSpace(text) == run.Name+":" {
		return transformer.CoTStep{}, false
	}
	return transformer.CoTStep{
		Index:      idx,
		StepType:   stepType,
		Text:       text,
		Confidence: run.Confidence,
	}, true
}

// FinalAnswer extracts the best answer from the completed agent runs.
func FinalAnswer(runs []transformer.AgentRun, query string) string {
	for i := len(runs) - 1; i >= 0; i-- {
		if runs[i].Role == "final answer" && strings.TrimSpace(runs[i].Output) != "" {
			return runs[i].Output
		}
	}
	for i := len(runs) - 1; i >= 0; i-- {
		if strings.TrimSpace(runs[i].Output) != "" {
			return runs[i].Output
		}
	}
	return fmt.Sprintf("Orchestration for %q produced no answer.", query)
}
