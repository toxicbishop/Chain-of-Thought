"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { doc, getDoc } from "firebase/firestore";
import { db } from "@/lib/firebase";
import { useCurrentUser } from "@/components/RequireAuth";
import { AgentGraph } from "@/components/AgentGraph";
import type { CoTStep, AgentRun, Delegation, TaskPlan } from "@/lib/api";
import { ArrowLeft, RefreshCw, Loader2, Clock, Brain, Terminal } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface TraceDoc {
  query: string;
  answer: string;
  steps: CoTStep[];
  agents: AgentRun[];
  delegations: Delegation[];
  plan: TaskPlan | null;
  cache_hit: boolean | null;
  duration_ms: number;
  created_at?: { toDate: () => Date } | null;
}

export default function Inspector() {
  const { id } = useParams<{ id: string }>();
  const user = useCurrentUser();
  const router = useRouter();
  const [trace, setTrace] = useState<TraceDoc | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedAgent, setSelectedAgent] = useState<string | null>(null);

  useEffect(() => {
    if (!user || !id) return;
    setLoading(true);
    getDoc(doc(db, "users", user.uid, "traces", id))
      .then((snap) => {
        if (!snap.exists()) {
          setError("Trace not found");
        } else {
          setTrace(snap.data() as TraceDoc);
        }
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [user, id]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="animate-spin text-muted-foreground" size={20} />
      </div>
    );
  }

  if (error || !trace) {
    return (
      <div className="p-4 md:p-8 max-w-3xl mx-auto">
        <Button variant="outline" size="sm" onClick={() => router.push("/history")}>
          <ArrowLeft size={14} className="mr-1" /> Back
        </Button>
        <div className="mt-6 p-8 bg-surface border border-border rounded-2xl text-center shadow-elegant">
          <p className="text-sm text-destructive">{error ?? "Trace unavailable"}</p>
        </div>
      </div>
    );
  }

  const selectedRun = trace.agents.find((a) => a.id === selectedAgent);

  return (
    <div className="p-3 sm:p-4 md:p-6 max-w-[1600px] mx-auto space-y-4">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <div className="flex items-start sm:items-center gap-3 min-w-0">
          <Button variant="ghost" size="sm" onClick={() => router.push("/history")}>
            <ArrowLeft size={14} className="mr-1" /> History
          </Button>
          <div>
            <h1 className="text-base sm:text-lg font-display font-semibold tracking-tight line-clamp-2 sm:truncate sm:max-w-2xl">
              {trace.query}
            </h1>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 mt-0.5 text-[11px] text-muted-foreground">
              {trace.created_at?.toDate && (
                <span className="flex items-center gap-1">
                  <Clock size={11} />
                  {trace.created_at.toDate().toLocaleString()}
                </span>
              )}
              <span className="font-mono">{(trace.duration_ms / 1000).toFixed(1)}s</span>
              <span className="font-mono">{trace.agents.length} agents</span>
              {trace.cache_hit !== null && (
                <span
                  className={cn(
                    "font-mono px-1.5 py-0.5 rounded border text-[10px]",
                    trace.cache_hit
                      ? "text-success border-success/30 bg-success/5"
                      : "text-primary border-primary/30 bg-primary/5"
                  )}
                >
                  {trace.cache_hit ? "CACHE HIT" : "CACHE MISS"}
                </span>
              )}
            </div>
          </div>
        </div>
        <Button
          onClick={() => {
            sessionStorage.setItem("cot_rerun_query", trace.query);
            router.push("/");
          }}
          className="bg-primary hover:bg-primary/90 text-primary-foreground border-0"
          size="sm"
        >
          <RefreshCw size={14} className="mr-1" /> Re-run
        </Button>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-5 gap-4">
        <section className="lg:col-span-3 bg-surface rounded-xl shadow-elegant border border-border overflow-hidden flex flex-col">
          <div className="px-4 py-3 border-b border-border bg-surface-raised/40">
            <h2 className="text-xs font-bold tracking-wider uppercase">Topology</h2>
          </div>
          <div className="bg-background h-[320px] sm:h-[380px] lg:h-[440px] relative">
            {trace.plan ? (
              <div className="absolute inset-0 p-2">
                <AgentGraph
                  plan={trace.plan}
                  runs={trace.agents}
                  delegations={trace.delegations}
                  activeId={null}
                  onNodeClick={setSelectedAgent}
                />
              </div>
            ) : (
              <div className="h-full flex items-center justify-center text-xs text-muted-foreground italic">
                No plan recorded for this trace
              </div>
            )}
          </div>
        </section>

        <section className="lg:col-span-2 bg-surface rounded-xl shadow-elegant border border-border overflow-hidden flex flex-col">
          <div className="px-4 py-3 border-b border-border bg-surface-raised/40">
            <h2 className="text-xs font-bold tracking-wider uppercase">Final answer</h2>
          </div>
          <div className="flex-1 overflow-y-auto p-4 min-h-[220px]">
            <p className="text-sm leading-relaxed whitespace-pre-wrap">{trace.answer}</p>
          </div>
        </section>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <section className="bg-surface rounded-xl shadow-elegant border border-border flex flex-col h-[340px] sm:h-[420px]">
          <div className="px-4 py-3 border-b border-border bg-surface-raised/40">
            <h2 className="text-xs font-bold tracking-wider uppercase">Agents</h2>
          </div>
          <div className="flex-1 overflow-y-auto divide-y divide-border">
            {trace.agents.map((a) => (
              <button
                key={a.id}
                onClick={() => setSelectedAgent(a.id)}
                className={cn(
                  "w-full text-left px-4 py-3 hover:bg-surface-raised/50 transition-colors",
                  selectedAgent === a.id && "bg-surface-raised"
                )}
              >
                <div className="flex items-center justify-between">
                  <span className="text-sm font-semibold">{a.name}</span>
                  <span
                    className={cn(
                      "text-[9px] font-mono px-1.5 py-0.5 rounded border",
                      a.status === "done"
                        ? "text-success border-success/30 bg-success/5"
                        : a.status === "failed"
                        ? "text-destructive border-destructive/30 bg-destructive/5"
                        : "text-muted-foreground border-border bg-surface-raised"
                    )}
                  >
                    {a.status.toUpperCase()}
                  </span>
                </div>
                <div className="text-[11px] text-muted-foreground mt-1 line-clamp-2">{a.task}</div>
              </button>
            ))}
          </div>
        </section>

        <section className="bg-surface rounded-xl shadow-elegant border border-border flex flex-col h-[340px] sm:h-[420px]">
          <div className="px-4 py-3 border-b border-border bg-surface-raised/40">
            <h2 className="text-xs font-bold tracking-wider uppercase">
              {selectedRun ? `Inspect: ${selectedRun.name}` : "Select an agent"}
            </h2>
          </div>
          <div className="flex-1 overflow-y-auto p-4 space-y-5">
            {selectedRun ? (
              <>
                <div>
                  <div className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider mb-1.5 flex items-center gap-1.5">
                    <Terminal size={11} /> Task
                  </div>
                  <div className="text-xs font-mono p-3 bg-surface-raised border border-border rounded-md leading-relaxed">
                    {selectedRun.task}
                  </div>
                </div>
                <div>
                  <div className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider mb-1.5 flex items-center gap-1.5">
                    <Brain size={11} /> Reasoning
                  </div>
                  <div className="text-sm italic p-3 border-l-2 border-primary bg-primary/5 rounded-r-md leading-relaxed">
                    {selectedRun.thought || "—"}
                  </div>
                </div>
                <div>
                  <div className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider mb-1.5">
                    Output
                  </div>
                  <div className="text-xs font-mono p-3 bg-background border border-border rounded-md text-success leading-relaxed">
                    {selectedRun.output || "—"}
                  </div>
                </div>
                {selectedRun.tool_calls && selectedRun.tool_calls.length > 0 && (
                  <div>
                    <div className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider mb-1.5">
                      Tool calls
                    </div>
                    <div className="space-y-2">
                      {selectedRun.tool_calls.map((tc, i) => (
                        <div key={i} className="bg-surface-raised border border-border rounded-md p-3 text-xs">
                          <div className="font-mono text-warning mb-1">{tc.name}()</div>
                          <pre className="text-[11px] overflow-x-auto">
                            {JSON.stringify(tc.inputs, null, 2)}
                          </pre>
                          <div className="mt-2 text-success">{tc.output}</div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </>
            ) : (
              <div className="h-full flex items-center justify-center text-xs text-muted-foreground italic">
                Click an agent on the left or in the topology
              </div>
            )}
          </div>
        </section>
      </div>

      {trace.steps.length > 0 && (
        <section className="bg-surface rounded-xl shadow-elegant border border-border overflow-hidden">
          <div className="px-4 py-3 border-b border-border bg-surface-raised/40">
            <h2 className="text-xs font-bold tracking-wider uppercase">Audit log</h2>
          </div>
          <div className="divide-y divide-border max-h-[400px] overflow-y-auto">
            {trace.steps.map((s, i) => (
              <div key={i} className="px-4 py-2.5 text-xs">
                <span className="font-mono text-muted-foreground mr-2">#{i + 1}</span>
                <span className="font-semibold text-primary mr-2">{s.step_type}</span>
                <span>{s.text}</span>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
