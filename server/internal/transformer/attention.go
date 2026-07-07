package transformer

import "math"

// AttentionLayer is one multi-head self-attention block.
type AttentionLayer struct {
	NumHeads int
	HeadDim  int
	EmbedDim int

	// Projection weights [EmbedDim x EmbedDim] (split into heads at runtime)
	Wq [][]float64
	Wk [][]float64
	Wv [][]float64
	Wo [][]float64 // output projection
}

func NewAttentionLayer(embedDim, numHeads int) *AttentionLayer {
	return &AttentionLayer{
		NumHeads: numHeads,
		HeadDim:  embedDim / numHeads,
		EmbedDim: embedDim,
		Wq:       randMatrix(embedDim, embedDim),
		Wk:       randMatrix(embedDim, embedDim),
		Wv:       randMatrix(embedDim, embedDim),
		Wo:       randMatrix(embedDim, embedDim),
	}
}

// Forward runs attention on X [seqLen x embedDim].
// Returns:
//   - output [seqLen x embedDim]
//   - snapshots  one AttentionSnapshot per head
func (a *AttentionLayer) Forward(X [][]float64, layerIdx int) ([][]float64, []AttentionSnapshot) {
	seqLen := len(X)
	scale := 1.0 / math.Sqrt(float64(a.HeadDim))

	// Linear projections → [seqLen x embedDim]
	Q := matmul(X, a.Wq)
	K := matmul(X, a.Wk)
	V := matmul(X, a.Wv)

	snapshots := make([]AttentionSnapshot, a.NumHeads)
	// Collect per-head context vectors to concat later
	allHeadOutputs := make([][][]float64, a.NumHeads) // [head][seqLen][headDim]

	for h := 0; h < a.NumHeads; h++ {
		start := h * a.HeadDim
		end := start + a.HeadDim

		// Slice head sub-matrices [seqLen x headDim]
		Qh := sliceCols(Q, start, end)
		Kh := sliceCols(K, start, end)
		Vh := sliceCols(V, start, end)

		// Attention scores [seqLen x seqLen]
		scores := matmul(Qh, transpose(Kh))
		// Scale
		for i := range scores {
			for j := range scores[i] {
				scores[i][j] *= scale
			}
		}
		// Causal mask
		scores = causalMask(scores)
		// Softmax
		attnWeights := softmax(scores)

		// Snapshot ← deep-copy weights
		wCopy := make([][]float64, seqLen)
		for i := range attnWeights {
			wCopy[i] = make([]float64, seqLen)
			copy(wCopy[i], attnWeights[i])
		}
		snapshots[h] = AttentionSnapshot{
			Layer:   layerIdx,
			Head:    h,
			Weights: wCopy,
		}

		// Context [seqLen x headDim]
		allHeadOutputs[h] = matmul(attnWeights, Vh)
	}

	// Concat heads → [seqLen x embedDim]
	concat := concatHeads(allHeadOutputs, seqLen, a.EmbedDim)
	// Output projection
	out := matmul(concat, a.Wo)
	return out, snapshots
}

// causalMask applies -inf to future positions.
func causalMask(scores [][]float64) [][]float64 {
	for i := range scores {
		for j := range scores[i] {
			if j > i {
				scores[i][j] = -1e9
			}
		}
	}
	return scores
}

// sliceCols returns columns [start:end] from matrix.
func sliceCols(M [][]float64, start, end int) [][]float64 {
	out := make([][]float64, len(M))
	for i, row := range M {
		out[i] = row[start:end]
	}
	return out
}

// concatHeads merges [head][seqLen][headDim] → [seqLen][embedDim].
func concatHeads(heads [][][]float64, seqLen, embedDim int) [][]float64 {
	out := make([][]float64, seqLen)
	for i := range out {
		out[i] = make([]float64, embedDim)
	}
	headDim := embedDim / len(heads)
	for h, head := range heads {
		for i := 0; i < seqLen; i++ {
			copy(out[i][h*headDim:(h+1)*headDim], head[i])
		}
	}
	return out
}
