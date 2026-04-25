package agents

import (
	"cot-backend/internal/llm"
	"cot-backend/internal/vectordb"
)

// Default returns the standard agent roster in a stable order.
// This is the only place agent wiring happens — add new agents here.
func Default(client *llm.Client, vdb *vectordb.Client) []Agent {
	return []Agent{
		NewResearcher(client, vdb),
		NewReasoner(client),
		NewCritic(client),
		NewSynthesizer(client),
		NewToolAgent(client),
	}
}
