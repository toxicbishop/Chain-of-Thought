package transformer

// FeedForward is a two-layer MLP with ReLU: [embedDim → ffDim → embedDim].
type FeedForward struct {
	W1 [][]float64 // [embedDim x ffDim]
	W2 [][]float64 // [ffDim x embedDim]
	B1 []float64
	B2 []float64
}

func NewFeedForward(embedDim, ffDim int) *FeedForward {
	return &FeedForward{
		W1: randMatrix(embedDim, ffDim),
		W2: randMatrix(ffDim, embedDim),
		B1: randVec(ffDim),
		B2: randVec(embedDim),
	}
}

func (ff *FeedForward) Forward(X [][]float64) [][]float64 {
	// hidden = relu(X @ W1 + B1)  [seqLen x ffDim]
	hidden := matmul(X, ff.W1)
	for i := range hidden {
		for j := range hidden[i] {
			hidden[i][j] += ff.B1[j]
		}
	}
	hidden = relu(hidden)
	// out = hidden @ W2 + B2  [seqLen x embedDim]
	out := matmul(hidden, ff.W2)
	for i := range out {
		for j := range out[i] {
			out[i][j] += ff.B2[j]
		}
	}
	return out
}

// TransformerBlock wraps attention + FF + residuals + layer-norms.
type TransformerBlock struct {
	Index     int
	Attention *AttentionLayer
	FF        *FeedForward
}

func NewTransformerBlock(idx, embedDim, numHeads, ffDim int) *TransformerBlock {
	return &TransformerBlock{
		Index:     idx,
		Attention: NewAttentionLayer(embedDim, numHeads),
		FF:        NewFeedForward(embedDim, ffDim),
	}
}

// BlockOutput holds the outputs and instrumentation from one transformer block.
type BlockOutput struct {
	Hidden     [][]float64 // [seqLen x embedDim]
	Snapshots  []AttentionSnapshot
	Activation LayerActivation
}

// Forward processes X through attention → residual → LN → FF → residual → LN.
// Captures attention weights and post-FF activation magnitudes.
func (b *TransformerBlock) Forward(X [][]float64) BlockOutput {
	// --- Self-attention ---
	attnOut, snapshots := b.Attention.Forward(X, b.Index)
	// Residual + LN
	afterAttn := layerNorm(add(X, attnOut), 1e-5)

	// --- Feed-forward ---
	ffOut := b.FF.Forward(afterAttn)
	// Residual + LN
	out := layerNorm(add(afterAttn, ffOut), 1e-5)

	// Activation capture: mean |activation| per token
	activation := LayerActivation{
		Layer:      b.Index,
		TokenMeans: meanAbsPerToken(out),
	}

	return BlockOutput{
		Hidden:     out,
		Snapshots:  snapshots,
		Activation: activation,
	}
}
