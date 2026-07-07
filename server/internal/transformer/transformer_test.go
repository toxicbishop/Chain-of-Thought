package transformer_test

import (
	"testing"

	"cot-backend/internal/transformer"
)

func TestDefaultConfig(t *testing.T) {
	cfg := transformer.DefaultConfig()
	if cfg.NumHeads == 0 || cfg.NumLayers == 0 {
		t.Fatal("invalid default config")
	}
	if cfg.EmbedDim%cfg.NumHeads != 0 {
		t.Fatalf("EmbedDim %d not divisible by NumHeads %d", cfg.EmbedDim, cfg.NumHeads)
	}
}

func TestTokenize(t *testing.T) {
	model := transformer.NewModel(transformer.DefaultConfig())
	ids, tokens := model.Tokenize("hello world reasoning")
	if len(ids) != 3 || len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(ids))
	}
	for _, id := range ids {
		if id < 0 || id >= transformer.DefaultConfig().VocabSize {
			t.Fatalf("token id %d out of vocab range", id)
		}
	}
}

func TestForwardShape(t *testing.T) {
	cfg := transformer.DefaultConfig()
	model := transformer.NewModel(cfg)
	ids, tokens := model.Tokenize("what is the capital of France")
	logits, snapshots, activations := model.Forward(ids, tokens)

	seqLen := len(ids)
	if len(logits) != seqLen {
		t.Fatalf("logits seqLen mismatch: got %d want %d", len(logits), seqLen)
	}
	if len(logits[0]) != cfg.VocabSize {
		t.Fatalf("logits vocabSize mismatch: got %d want %d", len(logits[0]), cfg.VocabSize)
	}

	expectedSnaps := cfg.NumLayers * cfg.NumHeads
	if len(snapshots) != expectedSnaps {
		t.Fatalf("attention snapshots: got %d want %d", len(snapshots), expectedSnaps)
	}

	if len(activations) != cfg.NumLayers {
		t.Fatalf("activations layers: got %d want %d", len(activations), cfg.NumLayers)
	}
	for i, act := range activations {
		if len(act.TokenMeans) != seqLen {
			t.Fatalf("layer %d activation token_means len: got %d want %d", i, len(act.TokenMeans), seqLen)
		}
	}
}

func TestAttentionWeightsSumToOne(t *testing.T) {
	cfg := transformer.DefaultConfig()
	model := transformer.NewModel(cfg)
	ids, tokens := model.Tokenize("step by step reasoning test")
	_, snapshots, _ := model.Forward(ids, tokens)

	for _, snap := range snapshots {
		for i, row := range snap.Weights {
			sum := 0.0
			for _, v := range row {
				sum += v
			}
			if sum < 0.99 || sum > 1.01 {
				t.Fatalf("layer %d head %d row %d attention weights sum to %f (want ~1.0)",
					snap.Layer, snap.Head, i, sum)
			}
		}
	}
}

func TestPipelineRun(t *testing.T) {
	model := transformer.NewModel(transformer.DefaultConfig())
	pipeline := transformer.NewPipeline(model)
	trace := pipeline.Run("explain how attention works")

	if trace.Query == "" {
		t.Fatal("trace.Query is empty")
	}
	if len(trace.CoTSteps) == 0 {
		t.Fatal("no CoT steps generated")
	}
	if trace.Answer == "" {
		t.Fatal("trace.Answer is empty")
	}
	if len(trace.Attentions) == 0 {
		t.Fatal("no attention snapshots captured")
	}
	if len(trace.Activations) == 0 {
		t.Fatal("no activations captured")
	}
	// At least one tool call expected (pipeline always generates one)
	if len(trace.ToolCalls) == 0 {
		t.Fatal("no tool calls captured")
	}
}

func TestCoTStepTypes(t *testing.T) {
	model := transformer.NewModel(transformer.DefaultConfig())
	pipeline := transformer.NewPipeline(model)
	trace := pipeline.Run("what is 2 + 2")

	validTypes := map[string]bool{
		"premise": true, "inference": true, "conclusion": true, "tool_call": true,
	}
	for _, step := range trace.CoTSteps {
		if !validTypes[step.StepType] {
			t.Fatalf("unknown step type: %q", step.StepType)
		}
		if step.Confidence < 0 || step.Confidence > 1 {
			t.Fatalf("confidence out of range: %f", step.Confidence)
		}
	}
}

// --- Benchmarks ---

func BenchmarkForwardPass(b *testing.B) {
	model := transformer.NewModel(transformer.DefaultConfig())
	ids, tokens := model.Tokenize("benchmark the transformer forward pass with a medium length sentence")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model.Forward(ids, tokens)
	}
}

func BenchmarkPipelineRun(b *testing.B) {
	model := transformer.NewModel(transformer.DefaultConfig())
	pipeline := transformer.NewPipeline(model)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pipeline.Run("benchmark query")
	}
}
