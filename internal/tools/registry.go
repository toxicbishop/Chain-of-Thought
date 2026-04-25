// Package tools provides a dynamic tool registry with schema validation.
// Tools are registered at startup and resolved at runtime by agents that
// need external capabilities (calculator, web search, code execution, etc.).
package tools

import (
	"context"
	"fmt"
	"sync"
)

// ParamSchema describes one parameter a tool accepts.
type ParamSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string" | "number" | "bool"
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ToolSchema describes a tool's contract — what it does, what it needs, and
// what it returns. This is what an agent "sees" when deciding whether to call.
type ToolSchema struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Params      []ParamSchema `json:"params"`
	ReturnType  string       `json:"return_type"` // "string" | "json"
}

// ToolFunc is the actual implementation. It receives validated inputs and
// returns a string result.
type ToolFunc func(ctx context.Context, inputs map[string]string) (string, error)

// Tool bundles a schema with its implementation.
type Tool struct {
	Schema ToolSchema
	Fn     ToolFunc
}

// Registry holds all registered tools and provides lookup + discovery.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]*Tool)}
}

// Register adds a tool to the registry. Panics on duplicate names (fail-fast
// at startup rather than silently overwriting).
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Schema.Name]; exists {
		panic(fmt.Sprintf("tool %q already registered", t.Schema.Name))
	}
	r.tools[t.Schema.Name] = &t
}

// Get returns a tool by name, or nil if not found.
func (r *Registry) Get(name string) *Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// Call looks up a tool by name, validates required parameters, and executes it.
func (r *Registry) Call(ctx context.Context, name string, inputs map[string]string) (string, error) {
	tool := r.Get(name)
	if tool == nil {
		return "", fmt.Errorf("tool %q not found in registry", name)
	}

	// Validate required params.
	for _, p := range tool.Schema.Params {
		if p.Required {
			if _, ok := inputs[p.Name]; !ok {
				return "", fmt.Errorf("tool %q: missing required param %q", name, p.Name)
			}
		}
	}

	return tool.Fn(ctx, inputs)
}

// List returns schemas for all registered tools. This is what agents use
// for capability discovery — "what tools are available to me?"
func (r *Registry) List() []ToolSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()
	schemas := make([]ToolSchema, 0, len(r.tools))
	for _, t := range r.tools {
		schemas = append(schemas, t.Schema)
	}
	return schemas
}

// Describe returns a human-readable manifest of all tools for injection
// into agent system prompts.
func (r *Registry) Describe() string {
	schemas := r.List()
	if len(schemas) == 0 {
		return "No tools available."
	}
	desc := "Available tools:\n"
	for _, s := range schemas {
		desc += fmt.Sprintf("- %s: %s\n", s.Name, s.Description)
		for _, p := range s.Params {
			req := ""
			if p.Required {
				req = " (required)"
			}
			desc += fmt.Sprintf("    param %q (%s): %s%s\n", p.Name, p.Type, p.Description, req)
		}
	}
	return desc
}
