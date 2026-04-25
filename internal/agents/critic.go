package agents

import "cot-backend/internal/llm"

// NewCritic audits the reasoner's output for errors or weak steps.
func NewCritic(client *llm.Client) Agent {
	return &llmAgent{
		name:       "Critic",
		role:       "verifier",
		capability: CapCritique,
		llm:        client,
		temp:       0.2,
		system: `You are a Critic agent. Audit the Reasoner's output for: factual errors,
unjustified leaps, missing cases, and ambiguity. Produce a pointed critique.
If the reasoning is sound, say so plainly — do not manufacture objections.
If a fact needs re-checking, delegate to capability "research".`,
	}
}
