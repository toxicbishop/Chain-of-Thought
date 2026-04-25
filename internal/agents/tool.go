package agents

import "cot-backend/internal/llm"

// NewToolAgent handles tool invocations (calculator, lookup, etc.).
// The current implementation is a stub that describes the tool call; real
// tool dispatch can be added later without changing the contract.
func NewToolAgent(client *llm.Client) Agent {
	return &llmAgent{
		name:       "ToolAgent",
		role:       "tool executor",
		capability: CapTool,
		llm:        client,
		temp:       0.0,
		system: `You are a ToolAgent. Given a request that needs a calculation or lookup,
produce the most likely result in "output" and record what tool you would have
called in "thought" (e.g. "calculator: 42*7"). Be deterministic and terse.`,
	}
}
