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

// ── Agent Color System ──────────────────────────────────────────────────────
// Each agent type gets a unique color for instant visual identification.
const AGENT_COLORS: Record<string, { primary: string; glow: string; bg: string; text: string }> = {
  research:   { primary: "#10B981", glow: "rgba(16,185,129,0.35)", bg: "rgba(16,185,129,0.08)",  text: "#059669" },
  reason:     { primary: "#6366F1", glow: "rgba(99,102,241,0.35)", bg: "rgba(99,102,241,0.08)",  text: "#4F46E5" },
  critique:   { primary: "#F59E0B", glow: "rgba(245,158,11,0.35)", bg: "rgba(245,158,11,0.08)",  text: "#D97706" },
  synthesize: { primary: "#EC4899", glow: "rgba(236,72,153,0.35)", bg: "rgba(236,72,153,0.08)",  text: "#DB2777" },
  tool:       { primary: "#8B5CF6", glow: "rgba(139,92,246,0.35)", bg: "rgba(139,92,246,0.08)",  text: "#7C3AED" },
};

const DEFAULT_COLOR = { primary: "#6366F1", glow: "rgba(99,102,241,0.35)", bg: "rgba(99,102,241,0.08)", text: "#4F46E5" };

function getAgentColor(capability: string) {
  return AGENT_COLORS[capability] ?? DEFAULT_COLOR;
}

// ── Status badge ────────────────────────────────────────────────────────────
const STATUS_ICONS: Record<AgentStatus, string> = {
  pending: "◯",
  running: "◉",
  done:    "✓",
  failed:  "✕",
};

function statusOf(
  taskId: string,
  runs: AgentRun[],
  activeId: string | null
): AgentStatus {
  const run = runs.find((r) => r.id === taskId);
  if (run) return run.status === "failed" ? "failed" : "done";
  if (activeId === taskId) return "running";
  return "pending";
}

export function AgentGraph({
  plan,
  runs,
  delegations,
  activeId,
  onNodeClick,
}: {
  plan: TaskPlan | null;
  runs: AgentRun[];
  delegations: Delegation[];
  activeId: string | null;
  onNodeClick?: (taskId: string) => void;
}) {
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
      const color = getAgentColor(t.agent);
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
              {/* Animated glow background for running state */}
              {isRunning && (
                <div
                  className="absolute inset-0 animate-pulse"
                  style={{
                    background: `radial-gradient(ellipse at center, ${color.glow}, transparent 70%)`,
                  }}
                />
              )}

              {/* Header: agent name + status badge */}
              <div className="flex items-center justify-between relative z-10 mb-1.5">
                <div className="flex items-center gap-1.5">
                  {/* Color dot */}
                  <div
                    className={`w-2 h-2 rounded-full ${isRunning ? "animate-pulse" : ""}`}
                    style={{
                      backgroundColor: color.primary,
                      boxShadow: isRunning ? `0 0 8px ${color.glow}` : "none",
                    }}
                  />
                  <span
                    className="text-[11px] font-bold tracking-tight"
                    style={{ color: color.text }}
                  >
                    {run?.name ?? t.agent.charAt(0).toUpperCase() + t.agent.slice(1)}
                  </span>
                </div>
                <span
                  className="text-[10px] font-mono"
                  style={{
                    color: isFailed
                      ? "#EF4444"
                      : isDone
                      ? color.primary
                      : "var(--muted)",
                  }}
                >
                  {STATUS_ICONS[status]}
                </span>
              </div>

              {/* Task description */}
              <div
                className="text-[9px] leading-tight relative z-10"
                style={{ color: "var(--muted)" }}
              >
                {t.task.length > 55 ? t.task.slice(0, 55) + "…" : t.task}
              </div>

              {/* Confidence bar */}
              {confidence != null && isDone && (
                <div className="mt-2 relative z-10">
                  <div
                    className="h-[3px] rounded-full overflow-hidden"
                    style={{ backgroundColor: "var(--border)" }}
                  >
                    <div
                      className="h-full rounded-full transition-all duration-700"
                      style={{
                        width: `${Math.round(confidence * 100)}%`,
                        backgroundColor: color.primary,
                      }}
                    />
                  </div>
                  <div
                    className="text-[8px] font-mono mt-0.5 text-right"
                    style={{ color: color.text }}
                  >
                    {Math.round(confidence * 100)}%
                  </div>
                </div>
              )}

              {/* Running progress */}
              {isRunning && (
                <div className="mt-2 h-[3px] rounded-full overflow-hidden relative z-10" style={{ backgroundColor: "var(--border)" }}>
                  <div
                    className="h-full rounded-full animate-[shimmer_1.5s_ease-in-out_infinite]"
                    style={{
                      width: "60%",
                      backgroundColor: color.primary,
                      backgroundImage: `linear-gradient(90deg, ${color.primary}, ${color.glow}, ${color.primary})`,
                      backgroundSize: "200% 100%",
                    }}
                  />
                </div>
              )}
            </div>
          ),
        },
        style: {
          background: "var(--surface)",
          border: `1.5px solid ${isDone || isRunning ? color.primary : isFailed ? "#EF4444" : "var(--border)"}`,
          borderRadius: "14px",
          width: 210,
          boxShadow: isRunning
            ? `0 0 20px ${color.glow}, 0 4px 12px rgba(0,0,0,0.1)`
            : isDone
            ? `0 0 8px ${color.glow}`
            : "0 2px 8px rgba(0,0,0,0.04)",
          cursor: "pointer",
          transition: "all 0.3s ease",
        },
      };
    });

    // Dependency edges
    const planEdges: Edge[] = [];
    for (const t of plan.tasks) {
      for (const dep of t.depends_on ?? []) {
        const depStatus = statusOf(dep, runs, activeId);
        const tStatus = statusOf(t.id, runs, activeId);
        const color = getAgentColor(t.agent);
        const isActive = depStatus === "done" && (tStatus === "running" || tStatus === "done");

        planEdges.push({
          id: `${dep}->${t.id}`,
          source: dep,
          target: t.id,
          animated: tStatus === "running",
          style: {
            stroke: isActive ? color.primary : "var(--border)",
            strokeWidth: isActive ? 2.5 : 1.5,
            opacity: isActive ? 1 : 0.5,
          },
          markerEnd: {
            type: MarkerType.ArrowClosed,
            color: isActive ? color.primary : "var(--border)",
            width: 16,
            height: 16,
          },
        });
      }
    }

    // Delegation Edges
    for (const d of delegations) {
      if (!d.task_id) continue;
      const sourceTask = d.task_id.split(".")[0];
      planEdges.push({
        id: `deleg-${sourceTask}-${d.task_id}`,
        source: sourceTask,
        target: d.task_id,
        animated: true,
        style: { stroke: "#8B5CF6", strokeWidth: 2, strokeDasharray: "6,4" },
        markerEnd: { type: MarkerType.ArrowClosed, color: "#8B5CF6" },
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
        <Background color="var(--border)" gap={24} size={1} />
        <Controls
          showInteractive={false}
          className="bg-(--surface) border border-(--border) shadow-sm rounded-lg overflow-hidden"
        />
      </ReactFlow>
    </div>
  );
}
