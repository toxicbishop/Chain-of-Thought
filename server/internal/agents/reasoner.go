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
task and produce a rigorous argument leading to a candidate answer.
Put the candidate answer in "output".

If retrieved source context directly answers the question, use that answer
instead of guessing from general knowledge or nearby labels. If source rows
conflict, prefer the explicit sentence that states the current project status.
If you need a calculation or external lookup, delegate to capability "tool".`,
	}
}
