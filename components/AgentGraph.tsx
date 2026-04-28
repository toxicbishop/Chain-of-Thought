"use client";

import { useMemo } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MarkerType,
  type Node,
  type Edge,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { TaskPlan, AgentRun, Delegation } from "@/lib/api";

type AgentStatus = "pending" | "running" | "done" | "failed";

// HSL strings (so they read from theme tokens at runtime)
const hsl = (cssVar: string) => `hsl(var(${cssVar}))`;
const hslA = (cssVar: string, a: number) => `hsl(var(${cssVar}) / ${a})`;

const AGENT_TOKENS: Record<string, string> = {
  research: "--agent-research",
  reason: "--agent-reason",
  critique: "--agent-critique",
  synthesize: "--agent-synthesize",
  tool: "--agent-tool",
};

function tokenFor(capability: string) {
  return AGENT_TOKENS[capability] ?? "--agent-reason";
}

const STATUS_ICONS: Record<AgentStatus, string> = {
  pending: "◯",
  running: "◉",
  done: "✓",
  failed: "✕",
};

function statusOf(taskId: string, runs: AgentRun[], activeId: string | null): AgentStatus {
  const run = runs.find((r) => r.id === taskId);
  if (run) return run.status === "failed" ? "failed" : "done";
  if (activeId === taskId) return "running";
  return "pending";
}

interface Props {
  plan: TaskPlan | null;
  runs: AgentRun[];
  delegations: Delegation[];
  activeId: string | null;
  onNodeClick?: (taskId: string) => void;
}

export function AgentGraph({ plan, runs, delegations, activeId, onNodeClick }: Props) {
  const { nodes, edges } = useMemo(() => {
    if (!plan) return { nodes: [] as Node[], edges: [] as Edge[] };

    const depth: Record<string, number> = {};
    for (const t of plan.tasks) {
      depth[t.id] = (t.depends_on ?? []).reduce(
        (d, dep) => Math.max(d, (depth[dep] ?? 0) + 1),
        0
      );
    }

    const perDepth: Record<number, number> = {};
    const planNodes: Node[] = plan.tasks.map((t) => {
      const d = depth[t.id];
      const idx = perDepth[d] ?? 0;
      perDepth[d] = idx + 1;

      const status = statusOf(t.id, runs, activeId);
      const cssVar = tokenFor(t.agent);
      const color = hsl(cssVar);
      const colorGlow = hslA(cssVar, 0.35);
      const run = runs.find((r) => r.id === t.id);
      const isRunning = status === "running";
      const isDone = status === "done";
      const isFailed = status === "failed";
      const confidence = run?.confidence;

      return {
        id: t.id,
        position: { x: d * 280 + 40, y: idx * 140 + 40 },
        data: {
          label: (
            <div className="p-3 text-left relative overflow-hidden">
              {isRunning && (
                <div
                  className="absolute inset-0 animate-pulse"
                  style={{
                    background: `radial-gradient(ellipse at center, ${colorGlow}, transparent 70%)`,
                  }}
                />
              )}

              <div className="flex items-center justify-between relative z-10 mb-1.5">
                <div className="flex items-center gap-1.5">
                  <div
                    className={`w-2 h-2 rounded-full ${isRunning ? "animate-pulse-dot" : ""}`}
                    style={{
                      backgroundColor: color,
                      boxShadow: isRunning ? `0 0 8px ${colorGlow}` : "none",
                    }}
                  />
                  <span className="text-[11px] font-bold tracking-tight" style={{ color }}>
                    {run?.name ?? t.agent.charAt(0).toUpperCase() + t.agent.slice(1)}
                  </span>
                </div>
                <span
                  className="text-[10px] font-mono"
                  style={{
                    color: isFailed
                      ? hsl("--destructive")
                      : isDone
                      ? color
                      : hsl("--muted-foreground"),
                  }}
                >
                  {STATUS_ICONS[status]}
                </span>
              </div>

              <div
                className="text-[10px] leading-tight relative z-10"
                style={{ color: hsl("--muted-foreground") }}
              >
                {t.task.length > 55 ? t.task.slice(0, 55) + "…" : t.task}
              </div>

              {confidence != null && isDone && (
                <div className="mt-2 relative z-10">
                  <div
                    className="h-[3px] rounded-full overflow-hidden"
                    style={{ backgroundColor: hsl("--border") }}
                  >
                    <div
                      className="h-full rounded-full transition-all duration-700"
                      style={{
                        width: `${Math.round(confidence * 100)}%`,
                        backgroundColor: color,
                      }}
                    />
                  </div>
                  <div className="text-[9px] font-mono mt-0.5 text-right" style={{ color }}>
                    {Math.round(confidence * 100)}%
                  </div>
                </div>
              )}

              {isRunning && (
                <div
                  className="mt-2 h-[3px] rounded-full overflow-hidden relative z-10"
                  style={{ backgroundColor: hsl("--border") }}
                >
                  <div
                    className="h-full rounded-full animate-shimmer"
                    style={{
                      width: "60%",
                      backgroundImage: `linear-gradient(90deg, ${color}, ${colorGlow}, ${color})`,
                      backgroundSize: "200% 100%",
                    }}
                  />
                </div>
              )}
            </div>
          ),
        },
        style: {
          background: hsl("--surface"),
          border: `1.5px solid ${
            isDone || isRunning ? color : isFailed ? hsl("--destructive") : hsl("--border")
          }`,
          borderRadius: "14px",
          width: 210,
          boxShadow: isRunning
            ? `0 0 24px ${colorGlow}, 0 4px 12px hsl(0 0% 0% / 0.2)`
            : isDone
            ? `0 0 12px ${colorGlow}`
            : "0 2px 8px hsl(0 0% 0% / 0.15)",
          cursor: "pointer",
          transition: "all 0.3s ease",
        } as React.CSSProperties,
      };
    });

    const planEdges: Edge[] = [];
    for (const t of plan.tasks) {
      for (const dep of t.depends_on ?? []) {
        const depStatus = statusOf(dep, runs, activeId);
        const tStatus = statusOf(t.id, runs, activeId);
        const cssVar = tokenFor(t.agent);
        const color = hsl(cssVar);
        const isActive = depStatus === "done" && (tStatus === "running" || tStatus === "done");

        planEdges.push({
          id: `${dep}->${t.id}`,
          source: dep,
          target: t.id,
          animated: tStatus === "running",
          style: {
            stroke: isActive ? color : hsl("--border-strong"),
            strokeWidth: isActive ? 2.5 : 1.5,
            opacity: isActive ? 1 : 0.5,
          },
          markerEnd: {
            type: MarkerType.ArrowClosed,
            color: isActive ? color : hsl("--border-strong"),
            width: 16,
            height: 16,
          },
        });
      }
    }

    for (const d of delegations) {
      if (!d.task_id) continue;
      const sourceTask = d.task_id.split(".")[0];
      planEdges.push({
        id: `deleg-${sourceTask}-${d.task_id}`,
        source: sourceTask,
        target: d.task_id,
        animated: true,
        style: {
          stroke: hsl("--accent"),
          strokeWidth: 2,
          strokeDasharray: "6,4",
        },
        markerEnd: { type: MarkerType.ArrowClosed, color: hsl("--accent") },
      });
    }

    return { nodes: planNodes, edges: planEdges };
  }, [plan, runs, delegations, activeId]);

  if (!plan) return null;

  return (
    <div className="w-full h-full min-h-[200px]">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        fitViewOptions={{ padding: 0.15 }}
        nodesDraggable={false}
        nodesConnectable={false}
        proOptions={{ hideAttribution: true }}
        onNodeClick={(_, node) => onNodeClick?.(node.id)}
      >
        <Background color={hsl("--border")} gap={24} size={1} />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}
