package transformer

import (
	"fmt"
	"math"
	"strings"
)

// Model is the full transformer with instrumented forward pass.
type Model struct {
	Config Config

	// Token embedding table [VocabSize x EmbedDim]
	Embedding [][]float64
	// Positional encoding [MaxSeqLen x EmbedDim] (sinusoidal, fixed)
	PosEnc [][]float64

	Blocks []*TransformerBlock

	// Output projection [EmbedDim x VocabSize]
	Wout [][]float64
}

func NewModel(cfg Config) *Model {
	blocks := make([]*TransformerBlock, cfg.NumLayers)
	for i := range blocks {
		blocks[i] = NewTransformerBlock(i, cfg.EmbedDim, cfg.NumHeads, cfg.FFDim)
	}
	m := &Model{
		Config:    cfg,
		Embedding: randMatrix(cfg.VocabSize, cfg.EmbedDim),
		PosEnc:    sinusoidalPE(cfg.MaxSeqLen, cfg.EmbedDim),
		Blocks:    blocks,
		Wout:      randMatrix(cfg.EmbedDim, cfg.VocabSize),
	}
	return m
}

// sinusoidalPE generates fixed sinusoidal positional encodings.
func sinusoidalPE(maxLen, dim int) [][]float64 {
	pe := make([][]float64, maxLen)
	for pos := range pe {
		pe[pos] = make([]float64, dim)
		for i := 0; i < dim; i += 2 {
			freq := 1.0 / math.Pow(10000, float64(i)/float64(dim))
			pe[pos][i] = math.Sin(float64(pos) * freq)
			if i+1 < dim {
				pe[pos][i+1] = math.Cos(float64(pos) * freq)
			}
		}
	}
	return pe
}

// Tokenize is a rudimentary whitespace + char tokenizer returning token IDs and strings.
func (m *Model) Tokenize(text string) ([]int, []string) {
	words := strings.Fields(text)
	ids := make([]int, len(words))
	for i, w := range words {
		// Hash word into vocab range deterministically
		h := 0
		for _, c := range w {
			h = (h*31 + int(c)) % m.Config.VocabSize
		}
		ids[i] = h
	}
	return ids, words
}

// Forward runs the full instrumented forward pass.
// Returns the full ReasoningTrace (minus CoT steps and tool calls — those are added by the pipeline).
func (m *Model) Forward(tokenIDs []int, tokens []string) ([][]float64, []AttentionSnapshot, []LayerActivation) {
	seqLen := len(tokenIDs)
	if seqLen > m.Config.MaxSeqLen {
		tokenIDs = tokenIDs[:m.Config.MaxSeqLen]
		tokens = tokens[:m.Config.MaxSeqLen]
		seqLen = m.Config.MaxSeqLen
	}

	// Build input embeddings + positional encodings
	X := make([][]float64, seqLen)
	for i, id := range tokenIDs {
		X[i] = make([]float64, m.Config.EmbedDim)
		for j := 0; j < m.Config.EmbedDim; j++ {
			X[i][j] = m.Embedding[id][j] + m.PosEnc[i][j]
		}
	}

	var allSnapshots []AttentionSnapshot
	var allActivations []LayerActivation

	// Pass through transformer blocks
	for _, block := range m.Blocks {
		res := block.Forward(X)
		X = res.Hidden
		allSnapshots = append(allSnapshots, res.Snapshots...)
		allActivations = append(allActivations, res.Activation)
	}

	// Logits [seqLen x VocabSize]
	logits := matmul(X, m.Wout)
	return logits, allSnapshots, allActivations
}

// Decode greedily picks the top token at the last position to produce next-token text.
func (m *Model) Decode(logits [][]float64) string {
	last := logits[len(logits)-1]
	best := 0
	for i, v := range last {
		if v > last[best] {
			best = i
		}
	}
	// Map vocab ID back to a pseudo-word
	return fmt.Sprintf("tok_%d", best)
}
