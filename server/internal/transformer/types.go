package transformer

// Config holds all hyperparameters for the transformer model.
type Config struct {
	VocabSize   int
	MaxSeqLen   int
	EmbedDim    int
	NumHeads    int
	NumLayers   int
	FFDim       int
	DropoutRate float64
}

func DefaultConfig() Config {
	return Config{
		VocabSize:   32000,
		MaxSeqLen:   512,
		EmbedDim:    256,
		NumHeads:    8,
		NumLayers:   6,
		FFDim:       1024,
		DropoutRate: 0.1,
	}
}

// ---- Shared data structures passed between layers and the API ----

// AttentionSnapshot captures one head's attention matrix at one layer.
type AttentionSnapshot struct {
	Layer   int         `json:"layer"`
	Head    int         `json:"head"`
	Weights [][]float64 `json:"weights"` // [SeqLen x SeqLen]
}

// LayerActivation captures the mean activation magnitude per token for a layer.
type LayerActivation struct {
	Layer      int       `json:"layer"`
	TokenMeans []float64 `json:"token_means"` // [SeqLen]
}

// CoTStep represents one structured reasoning step extracted from the model.
type CoTStep struct {
	Index      int     `json:"index"`
	StepType   string  `json:"step_type"` // "premise" | "inference" | "conclusion" | "tool_call"
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

// ToolCall represents an intermediate tool invocation captured during reasoning.
type ToolCall struct {
	Name   string            `json:"name"`
	Inputs map[string]string `json:"inputs"`
	Output string            `json:"output"`
}

// PlannedTask is one node in the orchestrator's DAG plan.
type PlannedTask struct {
	ID        string   `json:"id"`
	Agent     string   `json:"agent"`              // capability label the router resolves
	Task      string   `json:"task"`               // natural-language instruction
	DependsOn []string `json:"depends_on,omitempty"`
}

// TaskPlan is the orchestrator's decomposition of the user query.
type TaskPlan struct {
	Goal  string        `json:"goal"`
	Tasks []PlannedTask `json:"tasks"`
}

// AgentRun captures one agent's execution of one planned task.
type AgentRun struct {
	ID         string   `json:"id"`           // matches PlannedTask.ID
	Name       string   `json:"name"`         // concrete agent name
	Role       string   `json:"role"`         // human-readable role
	Task       string   `json:"task"`         // instruction executed
	Thought    string   `json:"thought"`      // internal reasoning
	Output     string   `json:"output"`       // result fed to downstream agents
	Confidence float64  `json:"confidence"`
	Status     string   `json:"status"`       // "done" | "failed" | "skipped"
	DependsOn  []string `json:"depends_on,omitempty"`
	StartedAt  string     `json:"started_at"`   // RFC3339
	EndedAt    string     `json:"ended_at"`     // RFC3339
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// Delegation records one agent handing sub-work to another.
type Delegation struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
	TaskID string `json:"task_id,omitempty"`
}

// ReasoningTrace is the full pipeline output for one query.
type ReasoningTrace struct {
	Query       string              `json:"query"`
	Answer      string              `json:"answer"`
	Tokens      []string            `json:"tokens"`
	CoTSteps    []CoTStep           `json:"cot_steps"`
	Attentions  []AttentionSnapshot `json:"attentions"`
	Activations []LayerActivation   `json:"activations"`
	ToolCalls   []ToolCall          `json:"tool_calls"`

	// Orchestrator-level fields. Populated when the request is routed through
	// the multi-agent orchestrator; omitted for raw transformer-only runs.
	Plan        *TaskPlan    `json:"plan,omitempty"`
	Agents      []AgentRun   `json:"agents,omitempty"`
	Delegations []Delegation `json:"delegations,omitempty"`
	Engine      string       `json:"engine,omitempty"` // "orchestrator" | "transformer"
}
