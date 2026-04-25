package agents

import (
	"context"
	"fmt"
	"strings"

	"cot-backend/internal/llm"
	"cot-backend/internal/vectordb"
)

// researcherAgent wraps llmAgent to inject VectorDB hybrid search results.
type researcherAgent struct {
	*llmAgent
	vdb *vectordb.Client
}

func (r *researcherAgent) Run(ctx context.Context, task Task) (Result, error) {
	if r.vdb != nil {
		vec, err := r.llm.Embed(ctx, task.Instruction)
		if err != nil {
			vec = nil // Fallback to pure BM25 search
		}

		results, err := r.vdb.HybridSearch(ctx, "Document", task.Instruction, vec, 3)
		if err == nil && len(results) > 0 {
			if task.Context == nil {
				task.Context = make(map[string]string)
			}
			var b strings.Builder
			for i, res := range results {
				b.WriteString(fmt.Sprintf("[%d] %s (score: %.2f)\n", i, res.Content, res.Score))
			}
			task.Context["hybrid_search_results"] = b.String()
		}
	}
	return r.llmAgent.Run(ctx, task)
}

// NewResearcher gathers relevant facts and sub-questions for a query.
func NewResearcher(client *llm.Client, vdb *vectordb.Client) Agent {
	base := &llmAgent{
		name:       "Researcher",
		role:       "fact gatherer",
		capability: CapResearch,
		llm:        client,
		temp:       0.3,
		system: `You are a Researcher agent. Your job is to surface the concrete facts,
definitions, sub-questions, and relevant background a reasoner will need to
answer the user's task. Be factual and concise. If a claim is uncertain, say so.
Do NOT attempt to answer the full question — only lay the groundwork. You will receive hybrid search results in your context.`,
	}
	return &researcherAgent{llmAgent: base, vdb: vdb}
}
