"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import {
  reasonStream,
  type CoTStep,
  type TaskPlan,
  type AgentRun,
  type Delegation,
  type AttentionSnapshot,
  type LayerActivation,
} from "@/lib/api";
import { useModelSettings } from "@/lib/hooks";
import { useCurrentUser } from "@/components/RequireAuth";
import { db } from "@/lib/firebase";
import { collection, addDoc, serverTimestamp } from "firebase/firestore";
import { AgentGraph } from "@/components/AgentGraph";
import {
  Brain,
  Terminal,
  Zap,
  X,
  LayoutGrid,
  Send,
  Loader2,
} from "lucide-react";
import { cn } from "@/lib/utils";

function Skeleton({ className }: { className?: string }) {
  return <div className={cn("animate-pulse bg-surface-raised rounded-md", className)} />;
}

const AGENT_TOKENS: Record<string, string> = {
  Researcher: "--agent-research",
  Reasoner: "--agent-reason",
  Critic: "--agent-critique",
  Synthesizer: "--agent-synthesize",
  ToolAgent: "--agent-tool",
};

function tokenFor(name: string) {
  return AGENT_TOKENS[name] ?? "--agent-reason";
}

function AuditLogEntry({ step, index }: { step: CoTStep; index: number }) {
  const colonIdx = step.text.indexOf(":");
  const agentName = colonIdx > 0 ? step.text.slice(0, colonIdx).trim() : "Agent";
  const output = colonIdx > 0 ? step.text.slice(colonIdx + 1).trim() : step.text;
  const tok = tokenFor(agentName);
  const dotColor = `hsl(var(${tok}))`;

  const stepLabel =
    step.step_type === "premise"
      ? "RESEARCH"
      : step.step_type === "conclusion"
      ? "SYNTHESIS"
      : step.step_type === "tool_call"
      ? "TOOL"
      : step.step_type.toUpperCase();

  return (
    <div className="flex gap-3 px-4 py-3 border-b border-border hover:bg-surface-raised/40 transition-all group animate-fade-in">
      <div className="flex flex-col items-center shrink-0">
        <div
          className="w-2.5 h-2.5 rounded-full border-2 mt-1 transition-all group-hover:scale-125"
          style={{ borderColor: dotColor, backgroundColor: `hsl(var(${tok}) / 0.25)` }}
        />
        <div className="w-px flex-1 bg-border mt-1" />
      </div>
      <div className="flex-1 min-w-0 pb-1">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-[10px] font-mono text-muted-foreground">#{index + 1}</span>
          <span
            className="text-[9px] font-bold px-1.5 py-0.5 rounded border"
            style={{
              backgroundColor: `hsl(var(${tok}) / 0.12)`,
              color: dotColor,
              borderColor: `hsl(var(${tok}) / 0.3)`,
            }}
          >
            {stepLabel}
          </span>
          <span className="text-[10px] font-semibold" style={{ color: dotColor }}>
            {agentName}
          </span>
        </div>
        <div className="text-xs text-foreground leading-relaxed truncate">{output}</div>
        <div className="flex items-center gap-3 mt-1">
          <div className="flex items-center gap-1">
            <div className="w-12 h-[3px] rounded-full bg-border overflow-hidden">
              <div
                className="h-full rounded-full transition-all duration-500"
                style={{
                  width: `${Math.round(step.confidence * 100)}%`,
                  backgroundColor: dotColor,
                }}
              />
            </div>
            <span className="text-[9px] font-mono text-muted-foreground">
              {Math.round(step.confidence * 100)}%
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}

function AgentDetailPanel({
  taskId,
  runs,
  plan,
  streaming,
  onClose,
}: {
  taskId: string | null;
  runs: AgentRun[];
  plan: TaskPlan | null;
  streaming: boolean;
  onClose: () => void;
}) {
  if (!taskId) return null;
  const run = runs.find((r) => r.id === taskId);
  const task = plan?.tasks.find((t) => t.id === taskId);

  return (
    <div className="fixed inset-y-0 right-0 w-full sm:w-[450px] bg-surface border-l border-border shadow-2xl z-50 flex flex-col animate-fade-in">
      <div className="p-6 border-b border-border flex items-center justify-between bg-surface-raised/40">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center border border-primary/20">
            <Brain className="text-primary" size={18} />
          </div>
          <div>
            <h3 className="text-sm font-bold">Agent Inspection</h3>
            <p className="text-[10px] text-muted-foreground uppercase tracking-wider">
              {run?.name ?? task?.agent} / {taskId}
            </p>
          </div>
        </div>
        <button
          onClick={onClose}
          className="p-2 hover:bg-surface-raised rounded-lg transition-colors text-muted-foreground"
        >
          <X size={20} />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-6 space-y-7">
        <section className="space-y-2">
          <h4 className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2">
            <Terminal size={12} /> Received prompt
          </h4>
          <div className="p-4 bg-surface-raised border border-border rounded-xl text-xs font-mono text-foreground leading-relaxed">
            {run?.task ?? task?.task}
          </div>
        </section>

        <section className="space-y-2">
          <h4 className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2">
            <Brain size={12} /> Internal reasoning
          </h4>
          <div className="p-4 border-l-2 border-primary bg-primary/5 rounded-r-xl text-sm italic text-foreground/80 leading-relaxed">
            {run?.thought || "Analysis in progress…"}
          </div>
        </section>

        {run?.tool_calls && run.tool_calls.length > 0 && (
          <section className="space-y-2">
            <h4 className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2">
              <Zap size={12} className="text-warning" /> Tool invocations
            </h4>
            <div className="space-y-3">
              {run.tool_calls.map((tc, i) => (
                <div
                  key={i}
                  className="bg-surface-raised border border-border rounded-xl overflow-hidden"
                >
                  <div className="px-3 py-2 bg-background text-[10px] font-mono flex justify-between border-b border-border">
                    <span className="text-warning">{tc.name}()</span>
                    <span className="text-muted-foreground">SUCCESS</span>
                  </div>
                  <div className="p-3 space-y-2">
                    <div className="text-[10px] text-muted-foreground uppercase">Input</div>
                    <pre className="text-[11px] font-mono text-foreground bg-background/50 p-2 rounded overflow-x-auto">
                      {JSON.stringify(tc.inputs, null, 2)}
                    </pre>
                    <div className="text-[10px] text-muted-foreground uppercase mt-2">Output</div>
                    <div className="text-xs text-foreground p-2 bg-success/5 rounded border border-success/20">
                      {tc.output}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </section>
        )}

        <section className="space-y-2">
          <h4 className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2">
            <LayoutGrid size={12} /> Output payload
          </h4>
          <div className="p-4 bg-background border border-border rounded-xl text-xs font-mono text-success overflow-x-auto leading-relaxed">
            {run?.output || (streaming ? "Generating output…" : "No output recorded.")}
          </div>
        </section>
      </div>

      <div className="p-4 border-t border-border bg-surface-raised/40 flex justify-between items-center text-[10px]">
        <div className="flex gap-4">
          <span className="text-muted-foreground">
            Status:{" "}
            <span
              className={
                run?.status === "done" ? "text-success" : run?.status === "failed" ? "text-destructive" : "text-warning"
              }
            >
              {run?.status ?? "pending"}
            </span>
          </span>
          <span className="text-muted-foreground">
            Confidence:{" "}
            <span className="text-primary">{((run?.confidence ?? 0) * 100).toFixed(1)}%</span>
          </span>
        </div>
        {run?.ended_at && (
          <span className="text-muted-foreground">
            {new Date(run.ended_at).toLocaleTimeString()}
          </span>
        )}
      </div>
    </div>
  );
}

export default function Workbench() {
  const user = useCurrentUser();
  const { settings: modelSettings } = useModelSettings();

  const [query, setQuery] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [steps, setSteps] = useState<CoTStep[]>([]);
  const [answer, setAnswer] = useState<string | null>(null);
  const [cacheHit, setCacheHit] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [plan, setPlan] = useState<TaskPlan | null>(null);
  const [agentRuns, setAgentRuns] = useState<AgentRun[]>([]);
  const [delegations, setDelegations] = useState<Delegation[]>([]);
  const [thoughts, setThoughts] = useState<{ id: string; name: string; thought: string }[]>([]);
  const [activeAgentId, setActiveAgentId] = useState<string | null>(null);
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [, setAttentions] = useState<AttentionSnapshot[]>([]);
  const [, setActivations] = useState<LayerActivation[]>([]);
  const startedAtRef = useRef<number>(0);

  const abortRef = useRef<AbortController | null>(null);

  // Pick up "Re-run" prompts handed off from the Inspector page
  useEffect(() => {
    const pending = sessionStorage.getItem("cot_rerun_query");
    if (pending) {
      sessionStorage.removeItem("cot_rerun_query");
      setQuery(pending);
    }
  }, []);

  const resetRun = useCallback(() => {
    setSteps([]);
    setAnswer(null);
    setCacheHit(null);
    setError(null);
    setPlan(null);
    setAgentRuns([]);
    setDelegations([]);
    setThoughts([]);
    setActiveAgentId(null);
    setSelectedTaskId(null);
    setAttentions([]);
    setActivations([]);
  }, []);

  const persistTrace = useCallback(
    async (finalAnswer: string, finalSteps: CoTStep[], finalRuns: AgentRun[]) => {
      if (!user) return;
      try {
        await addDoc(collection(db, "users", user.uid, "traces"), {
          query,
          answer: finalAnswer,
          steps: finalSteps,
          agents: finalRuns,
          delegations,
          plan,
          cache_hit: cacheHit,
          duration_ms: Date.now() - startedAtRef.current,
          created_at: serverTimestamp(),
        });
      } catch (err) {
        console.warn("Failed to persist trace:", err);
      }
    },
    [user, query, delegations, plan, cacheHit]
  );

  const handleStream = useCallback(async () => {
    if (!query.trim()) return;
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    setStreaming(true);
    resetRun();
    startedAtRef.current = Date.now();

    const collectedSteps: CoTStep[] = [];
    const collectedRuns: AgentRun[] = [];

    try {
      await reasonStream(
        query,
        (type, data) => {
          switch (type) {
            case "meta":
              setCacheHit((data as { cache_hit: boolean }).cache_hit);
              break;
            case "plan":
              setPlan(data as TaskPlan);
              break;
            case "agent_start":
              setActiveAgentId((data as { id: string }).id);
              break;
            case "agent_thought": {
              const t = data as { id: string; name: string; thought: string };
              setThoughts((prev) => [...prev, t]);
              break;
            }
            case "agent_done": {
              const run = data as AgentRun;
              collectedRuns.push(run);
              setAgentRuns((prev) => [...prev, run]);
              setActiveAgentId((curr) => (curr === run.id ? null : curr));
              if (run.status === "failed") {
                setThoughts((prev) => [
                  ...prev,
                  {
                    id: run.id,
                    name: `⚠ ${run.name}`,
                    thought: "LLM call timed out — used fallback stub for this step.",
                  },
                ]);
              }
              break;
            }
            case "delegation":
              setDelegations((prev) => [...prev, data as Delegation]);
              break;
            case "cot_step": {
              const s = data as CoTStep;
              collectedSteps.push(s);
              setSteps((prev) => [...prev, s]);
              break;
            }
            case "attention":
              setAttentions((prev) => [...prev, data as AttentionSnapshot]);
              break;
            case "activation":
              setActivations((prev) => [...prev, data as LayerActivation]);
              break;
            case "done": {
              const ans = (data as { answer: string }).answer;
              setAnswer(ans);
              setActiveAgentId(null);
              persistTrace(ans, collectedSteps, collectedRuns);
              break;
            }
          }
        },
        ctrl.signal,
        modelSettings
      );
    } catch (err: unknown) {
      if (err instanceof Error && err.name !== "AbortError") setError(err.message);
    } finally {
      setStreaming(false);
    }
  }, [query, resetRun, modelSettings, persistTrace]);

  return (
    <>
      <div className="p-3 sm:p-4 md:p-6 space-y-4 max-w-[1600px] mx-auto">
        {/* Query bar */}
        <section className="bg-surface rounded-xl shadow-elegant border border-border overflow-hidden">
          <div className="p-3 sm:p-4 flex flex-col sm:flex-row gap-3 sm:items-start">
            <div className="flex-1 relative">
              <textarea
                className="w-full bg-surface-raised border border-border rounded-lg p-3 text-sm leading-relaxed text-foreground placeholder:text-muted-foreground outline-none focus:ring-2 focus:ring-primary/30 focus:border-primary transition-all resize-none font-sans min-h-[88px] sm:min-h-[60px] max-h-[160px]"
                placeholder="Ask anything — the workbench will plan, delegate, reason, and synthesize. (⌘/Ctrl+Enter to run)"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (
                    (e.metaKey || e.ctrlKey) &&
                    e.key === "Enter" &&
                    !streaming &&
                    query.trim()
                  ) {
                    e.preventDefault();
                    handleStream();
                  }
                }}
              />
            </div>
            <button
              onClick={handleStream}
              disabled={streaming || !query.trim()}
              className="w-full sm:w-auto justify-center bg-primary text-primary-foreground px-5 py-3 rounded-lg text-sm font-semibold hover:bg-primary/90 shadow-elegant transition-colors disabled:opacity-50 whitespace-nowrap shrink-0 flex items-center gap-2"
            >
              {streaming ? (
                <>
                  <Loader2 size={14} className="animate-spin" />
                  Processing
                </>
              ) : (
                <>
                  <Send size={14} />
                  Run inference
                </>
              )}
            </button>
          </div>
          {error && (
            <div className="mx-4 mb-3 text-xs p-3 bg-destructive/10 border border-destructive/25 text-destructive rounded-lg animate-fade-in">
              {error}
            </div>
          )}
        </section>

        {/* Topology + Final answer */}
        <div className="grid grid-cols-1 lg:grid-cols-5 gap-4">
          <section className="lg:col-span-3 bg-surface rounded-xl shadow-elegant border border-border overflow-hidden flex flex-col">
            <div className="px-4 py-3 border-b border-border bg-surface-raised/40 flex items-center justify-between">
              <h2 className="text-xs font-bold tracking-wider uppercase">Process Topology</h2>
              {plan && (
                <span className="text-[10px] font-mono text-muted-foreground">
                  {plan.tasks.length} agents · {agentRuns.length} done
                </span>
              )}
            </div>
            <div className="bg-background min-h-[300px] h-[42vh] max-h-[460px] lg:h-[340px] flex-1 relative">
              {plan ? (
                <div className="absolute inset-0 p-2">
                  <AgentGraph
                    plan={plan}
                    runs={agentRuns}
                    delegations={delegations}
                    activeId={activeAgentId}
                    onNodeClick={setSelectedTaskId}
                  />
                </div>
              ) : streaming ? (
                <div className="w-full h-full flex flex-col sm:flex-row items-center justify-center gap-4 sm:gap-6 p-6">
                  <Skeleton className="w-32 h-14 sm:h-16 rounded-2xl" />
                  <Skeleton className="w-32 h-14 sm:h-16 rounded-2xl" />
                  <Skeleton className="w-32 h-14 sm:h-16 rounded-2xl" />
                </div>
              ) : (
                <div className="h-full flex flex-col items-center justify-center text-center space-y-3">
                  <div className="bg-surface-raised p-3 rounded-2xl border border-border">
                    <LayoutGrid className="text-muted-foreground" size={28} />
                  </div>
                  <div>
                    <p className="text-sm font-medium text-foreground">No active topology</p>
                    <p className="text-xs text-muted-foreground mt-1">
                      Enter a query above to spin up agents
                    </p>
                  </div>
                </div>
              )}
            </div>
          </section>

          <section className="lg:col-span-2 bg-surface rounded-xl shadow-elegant border border-border overflow-hidden flex flex-col">
            <div className="px-4 py-3 border-b border-border bg-surface-raised/40 flex items-center justify-between">
              <h2 className="text-xs font-bold tracking-wider uppercase">Final answer</h2>
              {cacheHit !== null && (
                <span
                  className={cn(
                    "text-[9px] font-mono px-2 py-0.5 rounded-full border",
                    cacheHit
                      ? "bg-success/10 text-success border-success/30"
                      : "bg-primary/10 text-primary border-primary/30"
                  )}
                >
                  {cacheHit ? "CACHE HIT" : "CACHE MISS"}
                </span>
              )}
            </div>
            <div className="flex-1 p-4 overflow-y-auto min-h-[240px] md:min-h-[280px]">
              {answer ? (
                <FormattedAnswer answer={answer} />
              ) : streaming ? (
                <div className="space-y-3 p-2">
                  <Skeleton className="h-4 w-full" />
                  <Skeleton className="h-4 w-[92%]" />
                  <Skeleton className="h-4 w-[75%]" />
                  <Skeleton className="h-4 w-[85%]" />
                  <Skeleton className="h-4 w-[60%]" />
                </div>
              ) : (
                <div className="h-full min-h-[200px] flex items-center justify-center text-muted-foreground text-xs font-mono italic">
                  Answer will appear here…
                </div>
              )}
            </div>
          </section>
        </div>

        {/* Audit log + Internal thoughts */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 pb-4">
          <section className="bg-surface rounded-xl shadow-elegant border border-border flex flex-col h-[300px] sm:h-[340px]">
            <div className="px-4 py-3 border-b border-border bg-surface-raised/40 flex items-center justify-between">
              <h2 className="text-xs font-bold tracking-wider uppercase">Technical Audit Log</h2>
              {steps.length > 0 && (
                <span className="text-[10px] font-mono text-muted-foreground">
                  {steps.length} steps
                </span>
              )}
            </div>
            <div className="flex-1 overflow-y-auto">
              {steps.length > 0 ? (
                steps.map((step, i) => <AuditLogEntry key={i} step={step} index={i} />)
              ) : streaming ? (
                <div className="p-4 space-y-4">
                  {[1, 2, 3].map((v) => (
                    <div key={v} className="flex items-center gap-3">
                      <Skeleton className="w-8 h-8 rounded-full shrink-0" />
                      <div className="flex-1 space-y-2">
                        <Skeleton className="h-3 w-3/4" />
                        <Skeleton className="h-2 w-1/4" />
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="h-full flex items-center justify-center text-muted-foreground text-xs font-mono italic">
                  Waiting for execution sequence…
                </div>
              )}
            </div>
          </section>

          <section className="bg-surface rounded-xl shadow-elegant border border-border flex flex-col h-[300px] sm:h-[340px]">
            <div className="px-4 py-3 border-b border-border bg-surface-raised/40 flex items-center justify-between">
              <h2 className="text-xs font-bold tracking-wider uppercase">Internal thoughts</h2>
              {thoughts.length > 0 && (
                <span className="text-[10px] font-mono text-muted-foreground">
                  {thoughts.length} steps
                </span>
              )}
            </div>
            <div className="flex-1 overflow-y-auto p-3">
              {thoughts.length > 0 ? (
                <div className="space-y-2">
                  {thoughts.map((t, i) => {
                    const tok = tokenFor(t.name);
                    const isWarning =
                      t.thought.includes("could not reach") ||
                      t.thought.includes("timed out") ||
                      t.thought.includes("fallback");
                    const isError =
                      t.thought.includes("error") || t.thought.includes("failed");

                    return (
                      <div
                        key={i}
                        className={cn(
                          "text-xs rounded-lg border animate-fade-in transition-all",
                          isWarning
                            ? "bg-warning/5 border-warning/25"
                            : isError
                            ? "bg-destructive/5 border-destructive/25"
                            : "bg-surface-raised border-border"
                        )}
                      >
                        <div className="flex items-center gap-2 px-3 pt-2.5 pb-1">
                          <div
                            className="w-1.5 h-1.5 rounded-full shrink-0"
                            style={{ backgroundColor: `hsl(var(${tok}))` }}
                          />
                          <span
                            className="text-[9px] font-bold px-1.5 py-0.5 rounded border"
                            style={{
                              backgroundColor: `hsl(var(${tok}) / 0.12)`,
                              color: `hsl(var(${tok}))`,
                              borderColor: `hsl(var(${tok}) / 0.3)`,
                            }}
                          >
                            STEP {i + 1}
                          </span>
                          <span
                            className="text-[10px] font-semibold"
                            style={{ color: `hsl(var(${tok}))` }}
                          >
                            {t.name}
                          </span>
                          {isWarning && <span className="text-[9px]">⚠</span>}
                        </div>
                        <div className="px-3 pb-2.5 pt-1 text-muted-foreground leading-relaxed">
                          {t.thought}
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : streaming ? (
                <div className="space-y-3">
                  {[1, 2].map((v) => (
                    <Skeleton key={v} className="h-12 w-full rounded-lg" />
                  ))}
                </div>
              ) : (
                <div className="h-full flex flex-col items-center justify-center text-center space-y-2">
                  <Brain className="text-border opacity-50" size={24} />
                  <span className="text-muted-foreground text-[11px] font-mono italic">
                    Agent reasoning will appear here…
                  </span>
                </div>
              )}
            </div>
          </section>
        </div>
      </div>

      <AgentDetailPanel
        taskId={selectedTaskId}
        runs={agentRuns}
        plan={plan}
        streaming={streaming}
        onClose={() => setSelectedTaskId(null)}
      />
    </>
  );
}

function FormattedAnswer({ answer }: { answer: string }) {
  let parsed: unknown = null;
  try {
    const trimmed = answer.trim();
    if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
      parsed = JSON.parse(trimmed);
    }
  } catch {
    /* not JSON */
  }

  if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
    return (
      <div className="animate-fade-in space-y-3">
        {Object.entries(parsed as Record<string, unknown>).map(([key, val]) => {
          const label = key.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
          if (Array.isArray(val)) {
            return (
              <div key={key}>
                <div className="text-[10px] font-bold text-primary uppercase tracking-wider mb-1.5">
                  {label}
                </div>
                <div className="space-y-2 pl-1">
                  {val.map((item, i) => (
                    <div
                      key={i}
                      className="text-sm text-foreground leading-relaxed border-l-2 border-primary/30 pl-3"
                    >
                      {typeof item === "object" ? (
                        <div>
                          {(item as { type?: string })?.type && (
                            <span className="font-semibold text-primary">
                              {(item as { type: string }).type}:{" "}
                            </span>
                          )}
                          <span>
                            {(item as { description?: string; text?: string }).description ||
                              (item as { text?: string }).text ||
                              JSON.stringify(item)}
                          </span>
                        </div>
                      ) : (
                        <span>{String(item)}</span>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            );
          }
          return (
            <div key={key}>
              <div className="text-[10px] font-bold text-primary uppercase tracking-wider mb-0.5">
                {label}
              </div>
              <p className="text-sm text-foreground leading-relaxed">{String(val)}</p>
            </div>
          );
        })}
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      <p className="text-sm text-foreground leading-relaxed whitespace-pre-wrap">{answer}</p>
    </div>
  );
}
