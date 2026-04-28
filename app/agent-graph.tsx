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
const AGENT_COLORS: Record<string, { primary: string; bg: string; text: string }> = {
  research:   { primary: "#27845D", bg: "rgba(39,132,93,0.10)",  text: "#1E6F4D" },
  reason:     { primary: "#1F7AAD", bg: "rgba(31,122,173,0.10)", text: "#17638E" },
  critique:   { primary: "#B97412", bg: "rgba(185,116,18,0.12)", text: "#925A0C" },
  synthesize: { primary: "#C43C5A", bg: "rgba(196,60,90,0.10)",  text: "#9F2F47" },
  tool:       { primary: "#208C85", bg: "rgba(32,140,133,0.10)", text: "#166F6A" },
};

const DEFAULT_COLOR = { primary: "#1F7AAD", bg: "rgba(31,122,173,0.10)", text: "#17638E" };

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
              {isRunning && (
                <div
                  className="absolute inset-y-0 left-0 w-1"
                  style={{
                    background: color.primary,
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
                    className="h-full rounded-full"
                    style={{
                      width: "60%",
                      backgroundColor: color.primary,
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
            ? `0 8px 18px rgba(0,0,0,0.14), inset 0 0 0 999px ${color.bg}`
            : isDone
            ? `0 8px 18px rgba(0,0,0,0.10), inset 0 0 0 999px ${color.bg}`
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
        style: { stroke: "#208C85", strokeWidth: 2, strokeDasharray: "6,4" },
        markerEnd: { type: MarkerType.ArrowClosed, color: "#208C85" },
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
