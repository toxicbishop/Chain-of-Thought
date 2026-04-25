package agents

import "cot-backend/internal/llm"

// NewReasoner performs the primary chain-of-thought analysis.
func NewReasoner(client *llm.Client) Agent {
	return &llmAgent{
		name:       "Reasoner",
		role:       "primary analyst",
		capability: CapReason,
		llm:        client,
		temp:       0.5,
		system: `You are a Reasoner agent. You take the Researcher's facts and the user's
task and produce a rigorous step-by-step argument leading to a candidate answer.
Show the inference chain explicitly in "thought". Put the candidate answer in "output".
If you need a calculation or external lookup, delegate to capability "tool".`,
	}
}
