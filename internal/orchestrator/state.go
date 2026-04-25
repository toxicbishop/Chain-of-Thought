// Package orchestrator – state.go
// ExecutionState provides shared memory across agent steps, output tracking,
// and guardrails (max iterations, critic loop detection, token budget).
package orchestrator

import (
	"fmt"
	"log"
	"sync"
)

// Guardrails defines safety limits for a single orchestration run.
type Guardrails struct {
	MaxAgentRuns   int // hard cap on total agent executions (incl. delegations)
	MaxCriticLoops int // max times the critic can re-trigger research
	MaxTokenBudget int // future: total token cap across all agents
}

// DefaultGuardrails returns sensible production defaults.
func DefaultGuardrails() Guardrails {
	return Guardrails{
		MaxAgentRuns:   12,
		MaxCriticLoops: 2,
		MaxTokenBudget: 50000,
	}
}

// ExecutionState tracks shared context across all agents in a single run.
// It is concurrency-safe — multiple goroutines (parallel DAG tasks) can
// read/write simultaneously.
type ExecutionState struct {
	mu sync.RWMutex

	// outputs maps taskID → agent output string.
	outputs map[string]string

	// memory is a key-value store agents can write to for cross-step state.
	// Example: researcher stores "key_facts" → synthesizer reads them.
	memory map[string]string

	// Counters for guardrail enforcement.
	agentRuns   int
	criticLoops int

	guards Guardrails
}

// NewExecutionState creates a fresh state with the given guardrails.
func NewExecutionState(g Guardrails) *ExecutionState {
	return &ExecutionState{
		outputs: make(map[string]string),
		memory:  make(map[string]string),
		guards:  g,
	}
}

// SetOutput records an agent's output by task ID.
func (s *ExecutionState) SetOutput(taskID, output string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outputs[taskID] = output
}

// GatherContext collects outputs from the specified dependency task IDs.
func (s *ExecutionState) GatherContext(depIDs []string) map[string]string {
	if len(depIDs) == 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx := make(map[string]string, len(depIDs))
	for _, id := range depIDs {
		if v, ok := s.outputs[id]; ok {
			ctx[id] = v
		}
	}
	return ctx
}

// SetMemory stores a value in shared agent memory.
func (s *ExecutionState) SetMemory(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memory[key] = value
}

// GetMemory retrieves a value from shared agent memory.
func (s *ExecutionState) GetMemory(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.memory[key]
	return v, ok
}

// MemorySnapshot returns a copy of the current memory map.
func (s *ExecutionState) MemorySnapshot() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.memory) == 0 {
		return nil
	}
	snap := make(map[string]string, len(s.memory))
	for k, v := range s.memory {
		snap[k] = v
	}
	return snap
}

// ── Guardrail Checks ────────────────────────────────────────────────────────

// RecordAgentRun increments the agent run counter. Returns an error if the
// guardrail limit has been exceeded.
func (s *ExecutionState) RecordAgentRun() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentRuns++
	if s.agentRuns > s.guards.MaxAgentRuns {
		log.Printf("[guardrail] max agent runs exceeded (%d/%d)", s.agentRuns, s.guards.MaxAgentRuns)
		return fmt.Errorf("guardrail: max agent runs (%d) exceeded", s.guards.MaxAgentRuns)
	}
	return nil
}

// RecordCriticLoop increments the critic re-trigger counter. Returns an error
// if the limit has been exceeded (prevents infinite critique→research loops).
func (s *ExecutionState) RecordCriticLoop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.criticLoops++
	if s.criticLoops > s.guards.MaxCriticLoops {
		log.Printf("[guardrail] max critic loops exceeded (%d/%d)", s.criticLoops, s.guards.MaxCriticLoops)
		return fmt.Errorf("guardrail: max critic loops (%d) exceeded", s.guards.MaxCriticLoops)
	}
	return nil
}

// Stats returns current execution statistics.
func (s *ExecutionState) Stats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]int{
		"agent_runs":   s.agentRuns,
		"critic_loops": s.criticLoops,
		"memory_keys":  len(s.memory),
		"outputs":      len(s.outputs),
	}
}
