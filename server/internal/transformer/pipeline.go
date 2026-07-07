package transformer

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
)

// Pipeline wraps the model with CoT generation and tool-call interception.
type Pipeline struct {
	Model *Model
}

func NewPipeline(model *Model) *Pipeline {
	return &Pipeline{Model: model}
}

// Run is the top-level entry point.  It:
//  1. Prepends a CoT system prompt to the query.
//  2. Runs the transformer forward pass (capturing attention + activations).
//  3. Parses the model output into structured CoT steps.
//  4. Intercepts any tool-call markers and generates synthetic tool responses.
//  5. Returns a complete ReasoningTrace.
// Run executes the pipeline without a context deadline (kept for backward compat).
func (p *Pipeline) Run(query string) ReasoningTrace {
	return p.RunWithContext(context.Background(), query)
}

// RunWithContext executes the pipeline, returning early if the context is cancelled.
func (p *Pipeline) RunWithContext(ctx context.Context, query string) ReasoningTrace {
	// Check cancellation before starting expensive work.
	select {
	case <-ctx.Done():
		return ReasoningTrace{Query: query, Answer: "request timed out"}
	default:
	}
	return p.run(query)
}

func (p *Pipeline) run(query string) ReasoningTrace {
	// 1. Construct CoT-guided prompt
	prompt := buildCoTPrompt(query)

	// 2. Tokenize
	tokenIDs, tokens := p.Model.Tokenize(prompt)

	// 3. Forward pass – capture everything
	logits, snapshots, activations := p.Model.Forward(tokenIDs, tokens)

	// 4. Generate a multi-step reasoning output (greedy, unrolled)
	generatedText := p.generateCoT(query, logits, tokens)

	// 5. Parse into structured steps + tool calls
	cotSteps, toolCalls := parseReasoningOutput(generatedText, activations)

	// 6. Final answer = last conclusion step or fallback
	answer := finalAnswer(cotSteps, query)

	return ReasoningTrace{
		Query:       query,
		Answer:      answer,
		Tokens:      tokens,
		CoTSteps:    cotSteps,
		Attentions:  snapshots,
		Activations: activations,
		ToolCalls:   toolCalls,
	}
}

// buildCoTPrompt prepends a structured thinking instruction.
// The query is sanitised to remove control tokens that could interfere
// with the prompt structure (e.g. injected [SYSTEM] or [ASSISTANT] tags).
func buildCoTPrompt(query string) string {
	safe := sanitiseQuery(query)
	return fmt.Sprintf(
		"[SYSTEM] Think step-by-step. Use <step> tags. If you need a tool write <tool>name:input</tool>.\n[USER] %s\n[ASSISTANT] Let me reason through this carefully.",
		safe,
	)
}

// sanitiseQuery strips prompt-control markers from user input so that a
// crafted query cannot break out of the [USER] section.
func sanitiseQuery(q string) string {
	for _, marker := range []string{"[SYSTEM]", "[USER]", "[ASSISTANT]"} {
		q = strings.ReplaceAll(q, marker, "")
	}
	// Also strip case-insensitive variants
	for _, marker := range []string{"[system]", "[user]", "[assistant]"} {
		for {
			idx := strings.Index(strings.ToLower(q), marker)
			if idx == -1 {
				break
			}
			q = q[:idx] + q[idx+len(marker):]
		}
	}
	return strings.TrimSpace(q)
}

// generateCoT simulates autoregressive generation of reasoning steps.
// In a real system this would be beam search; here we produce deterministic
// pseudo-steps seeded by the query token distribution.
func (p *Pipeline) generateCoT(query string, logits [][]float64, tokens []string) string {
	// Use last-position logit distribution to seed step diversity
	last := logits[len(logits)-1]
	topK := topKIndices(last, 5)

	words := strings.Fields(query)
	if len(words) == 0 {
		words = []string{"the", "problem"}
	}

	steps := []string{
		fmt.Sprintf("<step type='premise'>The question asks about: %s</step>", query),
		fmt.Sprintf("<step type='inference'>Breaking this into parts: token distribution peaks at vocab id %d</step>", topK[0]),
		fmt.Sprintf("<step type='tool_call'><tool>calculator:%s+analysis</tool></step>", words[0]),
		fmt.Sprintf("<step type='inference'>Applying reasoning over %d tokens with %d attention heads</step>",
			len(tokens), p.Model.Config.NumHeads),
		fmt.Sprintf("<step type='inference'>Layer activations suggest focus on position %d</step>",
			maxActivationPos(logits)),
		fmt.Sprintf("<step type='conclusion'>Therefore the answer to '%s' follows from the above analysis.</step>", query),
	}
	return strings.Join(steps, "\n")
}

// parseReasoningOutput converts the raw generated text into CoTStep and ToolCall slices.
func parseReasoningOutput(text string, activations []LayerActivation) ([]CoTStep, []ToolCall) {
	lines := strings.Split(text, "\n")
	var steps []CoTStep
	var tools []ToolCall
	stepIdx := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		stepType := extractAttr(line, "type")
		if stepType == "" {
			stepType = "inference"
		}

		// Strip XML tags for display text
		content := stripTags(line)

		// Confidence: derived from mean activation magnitude at the corresponding layer
		confidence := 0.5
		if stepIdx < len(activations) {
			confidence = clamp(meanFloat(activations[stepIdx].TokenMeans)*2, 0.1, 0.99)
		}

		if stepType == "tool_call" {
			tc := extractToolCall(content)
			tools = append(tools, tc)
		}

		steps = append(steps, CoTStep{
			Index:      stepIdx,
			StepType:   stepType,
			Text:       content,
			Confidence: confidence,
		})
		stepIdx++
	}
	return steps, tools
}

func extractToolCall(text string) ToolCall {
	// Expect pattern "name:input" inside <tool>…</tool>
	inner := between(text, "<tool>", "</tool>")
	parts := strings.SplitN(inner, ":", 2)
	name, input := "unknown", ""
	if len(parts) >= 1 {
		name = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		input = strings.TrimSpace(parts[1])
	}
	return ToolCall{
		Name:   name,
		Inputs: map[string]string{"query": input},
		Output: fmt.Sprintf("[simulated result for %s(%s)]", name, input),
	}
}

func finalAnswer(steps []CoTStep, query string) string {
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].StepType == "conclusion" {
			return steps[i].Text
		}
	}
	return fmt.Sprintf("Analysis of '%s' completed via %d reasoning steps.", query, len(steps))
}

// --- Helpers ---

func topKIndices(v []float64, k int) []int {
	indices := make([]int, k)
	used := map[int]bool{}
	for i := 0; i < k; i++ {
		best := -1
		for j := range v {
			if !used[j] && (best == -1 || v[j] > v[best]) {
				best = j
			}
		}
		indices[i] = best
		used[best] = true
	}
	return indices
}

func maxActivationPos(logits [][]float64) int {
	best := 0
	bestVal := math.Inf(-1)
	for i, row := range logits {
		for _, v := range row {
			if v > bestVal {
				bestVal = v
				best = i
			}
		}
	}
	return best
}

func meanFloat(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func extractAttr(s, attr string) string {
	key := attr + "='"
	start := strings.Index(s, key)
	if start == -1 {
		return ""
	}
	start += len(key)
	end := strings.Index(s[start:], "'")
	if end == -1 {
		return ""
	}
	return s[start : start+end]
}

func stripTags(s string) string {
	inTag := false
	var b strings.Builder
	for _, c := range s {
		if c == '<' {
			inTag = true
		} else if c == '>' {
			inTag = false
		} else if !inTag {
			b.WriteRune(c)
		}
	}
	return strings.TrimSpace(b.String())
}

func between(s, open, close string) string {
	start := strings.Index(s, open)
	if start == -1 {
		return s
	}
	start += len(open)
	end := strings.Index(s[start:], close)
	if end == -1 {
		return s[start:]
	}
	return s[start : start+end]
}

// Silence unused import warning for rand (used in randMatrix via math.go)
var _ = rand.Float64
