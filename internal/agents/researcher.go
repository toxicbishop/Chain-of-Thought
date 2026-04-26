package agents

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"cot-backend/internal/llm"
	"cot-backend/internal/vectordb"
)

// researcherAgent wraps llmAgent to inject VectorDB hybrid search results.
type researcherAgent struct {
	*llmAgent
	vdb *vectordb.Client
}

var projectLevelPattern = regexp.MustCompile(`(?i)\b(?:project\s+\S+\s+)?is at the ([A-Za-z][A-Za-z -]*? level)\b`)

func (r *researcherAgent) Run(ctx context.Context, task Task) (Result, error) {
	if r.vdb != nil {
		vec, err := r.llm.Embed(ctx, task.Instruction)
		if err != nil {
			vec = nil // Fallback to pure BM25 search.
		}

		results, err := r.vdb.HybridSearch(ctx, "Document", task.Instruction, vec, 5)
		if err == nil && len(results) > 0 {
			if task.Context == nil {
				task.Context = make(map[string]string)
			}
			var b strings.Builder
			for i, res := range results {
				citation := res.Source
				if citation == "" {
					citation = "knowledge-base"
				}
				if direct := directProjectLevelAnswer(task.Instruction, res.Content, citation, res.ChunkIndex); direct != "" {
					task.Context["source_direct_answer"] = direct
				}
				b.WriteString(fmt.Sprintf("[%d] %s#chunk-%d score=%.2f\n%s\n", i+1, citation, res.ChunkIndex, res.Score, res.Content))
			}
			task.Context["hybrid_search_results"] = b.String()
		}
	}
	return r.llmAgent.Run(ctx, task)
}

func directProjectLevelAnswer(question, content, source string, chunkIndex int) string {
	q := strings.ToLower(question)
	if !strings.Contains(q, "level") && !strings.Contains(q, "stage") && !strings.Contains(q, "status") {
		return ""
	}
	matches := projectLevelPattern.FindStringSubmatch(content)
	if len(matches) < 2 {
		return ""
	}
	return fmt.Sprintf("Direct source answer: the project is at the %s. Citation: %s#chunk-%d.", matches[1], source, chunkIndex)
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
Do NOT attempt to answer the full question; only lay the groundwork.

You may receive hybrid search results in your context. Treat those as the
highest-priority source of truth. When they directly answer the user's question,
put the exact relevant fact first in your output and include the source/chunk
label. Do not infer a different answer from nearby rows or related labels.`,
	}
	return &researcherAgent{llmAgent: base, vdb: vdb}
}
