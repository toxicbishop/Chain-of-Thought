// Package orchestrator – dag.go
// DAG executor that resolves task dependencies and runs independent agents
// in parallel. This replaces the linear for-loop with a proper graph runtime.
package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"cot-backend/internal/agents"
	"cot-backend/internal/transformer"
)

// dagResult is the output of a single task within the DAG.
type dagResult struct {
	TaskID string
	Run    transformer.AgentRun
	Res    agents.Result
}

// ExecuteDAG runs all planned tasks respecting dependency order, executing
// independent tasks concurrently. It returns completed AgentRuns in
// topological (finish) order.
func (o *Orchestrator) ExecuteDAG(
	ctx context.Context,
	plan transformer.TaskPlan,
	state *ExecutionState,
	events chan<- Event,
) []dagResult {
	// Build adjacency: which tasks depend on which.
	pending := make(map[string]*transformer.PlannedTask, len(plan.Tasks))
	depCount := make(map[string]int, len(plan.Tasks))       // remaining unmet deps
	dependents := make(map[string][]string)                  // taskID → tasks that depend on it

	for i := range plan.Tasks {
		pt := &plan.Tasks[i]
		pending[pt.ID] = pt
		depCount[pt.ID] = len(pt.DependsOn)
		for _, dep := range pt.DependsOn {
			dependents[dep] = append(dependents[dep], pt.ID)
		}
	}

	// Channels for coordination.
	resultCh := make(chan dagResult, len(plan.Tasks))
	var results []dagResult
	var wg sync.WaitGroup

	// Seed: launch all tasks with zero dependencies.
	for id, count := range depCount {
		if count == 0 {
			wg.Add(1)
			go o.dispatchTask(ctx, pending[id], state, events, resultCh, &wg)
		}
	}

	// Closer goroutine: when all tasks finish, close the result channel.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Consume results and unblock dependents.
	for dr := range resultCh {
		results = append(results, dr)
		state.SetOutput(dr.TaskID, dr.Run.Output)

		// Decrement dep count for all tasks that depended on this one.
		for _, childID := range dependents[dr.TaskID] {
			depCount[childID]--
			if depCount[childID] == 0 {
				wg.Add(1)
				go o.dispatchTask(ctx, pending[childID], state, events, resultCh, &wg)
			}
		}
	}

	return results
}

// dispatchTask runs a single planned task and sends the result on ch.
func (o *Orchestrator) dispatchTask(
	ctx context.Context,
	pt *transformer.PlannedTask,
	state *ExecutionState,
	events chan<- Event,
	ch chan<- dagResult,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	if ctx.Err() != nil {
		return
	}

	agent := o.route(pt.Agent)
	task := agents.Task{
		ID:          pt.ID,
		Instruction: pt.Task,
		Context:     state.GatherContext(pt.DependsOn),
	}

	// Inject shared memory into the agent's context.
	if mem := state.MemorySnapshot(); len(mem) > 0 {
		if task.Context == nil {
			task.Context = make(map[string]string)
		}
		for k, v := range mem {
			task.Context[fmt.Sprintf("memory:%s", k)] = v
		}
	}

	run, res := o.executor.RunAgent(ctx, agent, task, pt.DependsOn, events)
	ch <- dagResult{TaskID: pt.ID, Run: run, Res: res}
}
