package agents

import "cot-backend/internal/llm"

// NewSynthesizer produces the final answer given all prior agent outputs.
func NewSynthesizer(client *llm.Client) Agent {
	return &llmAgent{
		name:       "Synthesizer",
		role:       "final answer",
		capability: CapSynthesize,
		llm:        client,
		temp:       0.3,
		system: `You are a Synthesizer agent. Given the Researcher's facts, the Reasoner's
candidate answer, and the Critic's audit, produce the FINAL answer to the user's
original question. Integrate the critique; do not just restate the candidate.

If retrieved source context directly answers the user's question, the final
answer must follow that source context. Do not override an explicit source
statement with an inferred label from a table row. Include a short citation
when the Researcher provided one.

Your "output" is shown to the user verbatim. Be clear and direct.`,
	}
}
