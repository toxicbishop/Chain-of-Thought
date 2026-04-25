"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { signIn, signOut, onAuthChange } from "@/lib/firebase";
import {
  useQueryHistory,
  useLiveClock,
  useHealthCheck,
  useSessionCounter,
  useTheme,
  useModelSettings,
  useTelemetry,
} from "@/lib/hooks";
import type { User } from "firebase/auth";
import {
  reason,
  reasonStream,
  type ReasoningTrace,
  type CoTStep,
  type TaskPlan,
  type AgentRun,
  type Delegation,
  type ToolCall,
  type AttentionSnapshot,
  type LayerActivation,
} from "@/lib/api";
import { AgentGraph } from "./agent-graph";
import {
  Search,
  History,
  Settings,
  Layers,
  Activity,
  ChevronRight,
  CheckCircle2,
  Terminal,
  LogOut,
  Moon,
  Sun,
  LayoutGrid,
  Zap,
  Menu,
  X,
  Server,
  Info,
  Brain,
} from "lucide-react";

// ── Auth gate ──────────────────────────────────

function AuthForm({ onAuth }: { onAuth: () => void }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await signIn(email, password);
      onAuth();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Sign-in failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-(--background) px-4">
      <div className="w-full max-w-md bg-(--surface) p-6 sm:p-10 shadow-2xl rounded-xl border border-(--border)">
        <div className="flex justify-center mb-8">
          <div className="bg-indigo-600 p-3 rounded-xl shadow-lg shadow-indigo-200">
            <Zap className="text-white" size={32} />
          </div>
        </div>
        <h2 className="text-2xl font-bold text-(--foreground) text-center mb-2">
          Workbench Access
        </h2>
        <p className="text-(--muted) text-center mb-8 text-sm">
          Please authenticate to access the Logic Flow workbench.
        </p>

        <form onSubmit={handleSubmit} className="space-y-5">
          <div className="space-y-2">
            <label className="text-xs font-semibold text-slate-600 uppercase tracking-wider ml-1">
              Identity
            </label>
            <input
              type="email"
              required
              placeholder="operator@logicflow.io"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full bg-(--surface-raised) border border-slate-200 rounded-lg px-4 py-3 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition-all outline-none"
            />
          </div>
          <div className="space-y-2">
            <label className="text-xs font-semibold text-slate-600 uppercase tracking-wider ml-1">
              Credential
            </label>
            <input
              type="password"
              required
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full bg-(--surface-raised) border border-slate-200 rounded-lg px-4 py-3 text-sm focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition-all outline-none"
            />
          </div>

          {error && (
            <div className="text-xs p-3 bg-red-50 border border-red-100 text-red-600 rounded-lg">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-indigo-600 text-white py-3 rounded-lg font-bold text-sm hover:bg-indigo-700 shadow-lg shadow-indigo-100 transition-all disabled:opacity-50">
            {loading ? "Initializing..." : "Establish Connection"}
          </button>
        </form>
      </div>
    </div>
  );
}

function Skeleton({ className }: { className?: string }) {
  return (
    <div
      className={`animate-pulse bg-(--surface-raised) rounded-lg ${className}`}
    />
  );
}

// ── Components ──────────────────────────────────

function AttentionHeatmap({
  snapshots,
  tokens,
}: {
  snapshots: AttentionSnapshot[];
  tokens?: string[];
}) {
  if (snapshots.length === 0) return null;
  const latest = snapshots[snapshots.length - 1];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h4 className="text-[10px] font-bold text-(--muted) uppercase tracking-widest">
          Attention Heatmap (Layer 0)
        </h4>
        <div className="flex gap-1">
          {[1, 2, 3, 4].map((v) => (
            <div
              key={v}
              className="w-2 h-2 rounded-full"
              style={{ background: `rgba(99, 102, 241, ${v * 0.25})` }}
            />
          ))}
        </div>
      </div>
      <div className="grid grid-cols-16 gap-px bg-(--border) p-px rounded-lg overflow-hidden border border-(--border)">
        {latest.weights
          .slice(0, 16)
          .map((row, i) =>
            row
              .slice(0, 16)
              .map((val, j) => (
                <div
                  key={`${i}-${j}`}
                  className="aspect-square bg-indigo-500 transition-all duration-300"
                  style={{ opacity: Math.min(val * 8, 1) }}
                  title={`Pos ${i},${j}: ${val.toFixed(4)}`}
                />
              )),
          )}
      </div>
      <p className="text-[10px] text-(--muted) italic">
        Live weights captured from custom transformer during inference stream.
      </p>
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
    <div className="fixed inset-y-0 right-0 w-full sm:w-[450px] bg-(--surface) border-l border-(--border) shadow-2xl z-50 flex flex-col animate-in slide-in-from-right duration-300">
      <div className="p-6 border-b border-(--border) flex items-center justify-between bg-(--surface-raised)/30">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-indigo-500/10 flex items-center justify-center border border-indigo-500/20">
            <Brain className="text-indigo-500" size={18} />
          </div>
          <div>
            <h3 className="text-sm font-bold text-(--foreground)">
              Agent Inspection
            </h3>
            <p className="text-[10px] text-(--muted) uppercase tracking-wider">
              {run?.name ?? task?.agent} / {taskId}
            </p>
          </div>
        </div>
        <button
          onClick={onClose}
          className="p-2 hover:bg-(--surface-raised) rounded-lg transition-colors">
          <X size={20} className="text-(--muted)" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-6 space-y-8">
        {/* Instruction Section */}
        <section className="space-y-3">
          <h4 className="text-[10px] font-bold text-(--muted) uppercase tracking-widest flex items-center gap-2">
            <Terminal size={12} /> Received Prompt
          </h4>
          <div className="p-4 bg-(--surface-raised) border border-(--border) rounded-xl text-xs font-mono text-slate-300 leading-relaxed">
            {run?.task ?? task?.task}
          </div>
        </section>

        {/* Thought Section */}
        <section className="space-y-3">
          <h4 className="text-[10px] font-bold text-(--muted) uppercase tracking-widest flex items-center gap-2">
            <Brain size={12} /> Internal Reasoning
          </h4>
          <div className="p-4 border-l-2 border-indigo-500 bg-indigo-500/5 rounded-r-xl text-sm italic text-indigo-100/80 leading-relaxed">
            {run?.thought || "Analysis in progress..."}
          </div>
        </section>

        {/* Tool Calls */}
        {run?.tool_calls && run.tool_calls.length > 0 && (
          <section className="space-y-3">
            <h4 className="text-[10px] font-bold text-(--muted) uppercase tracking-widest flex items-center gap-2">
              <Zap size={12} className="text-amber-400" /> Tool Invocations
            </h4>
            <div className="space-y-3">
              {run.tool_calls.map((tc, i) => (
                <div
                  key={i}
                  className="bg-(--surface-raised) border border-(--border) rounded-xl overflow-hidden">
                  <div className="px-3 py-2 bg-slate-800 text-[10px] font-mono flex justify-between">
                    <span className="text-amber-400">{tc.name}()</span>
                    <span className="text-slate-500">SUCCESS</span>
                  </div>
                  <div className="p-3 space-y-2">
                    <div className="text-[10px] text-slate-500 uppercase">
                      Input
                    </div>
                    <pre className="text-[11px] font-mono text-slate-300 bg-black/20 p-2 rounded">
                      {JSON.stringify(tc.inputs, null, 2)}
                    </pre>
                    <div className="text-[10px] text-slate-500 uppercase mt-2">
                      Output
                    </div>
                    <div className="text-xs text-slate-300 p-2 bg-green-500/5 rounded border border-green-500/10">
                      {tc.output}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </section>
        )}

        {/* Payload Section */}
        <section className="space-y-3">
          <h4 className="text-[10px] font-bold text-(--muted) uppercase tracking-widest flex items-center gap-2">
            <LayoutGrid size={12} /> Output Payload
          </h4>
          <div className="p-4 bg-slate-900 border border-slate-800 rounded-xl text-xs font-mono text-emerald-400 overflow-x-auto">
            {run?.output ||
              (streaming ? "Generating output..." : "No output recorded.")}
          </div>
        </section>
      </div>

      <div className="p-6 border-t border-(--border) bg-(--surface-raised)/30 flex justify-between items-center text-[10px]">
        <div className="flex gap-4">
          <span className="text-(--muted)">
            Status:{" "}
            <span
              className={
                run?.status === "done" ? "text-green-500" : "text-amber-500"
              }>
              {run?.status ?? "pending"}
            </span>
          </span>
          <span className="text-(--muted)">
            Confidence:{" "}
            <span className="text-indigo-400">
              {((run?.confidence ?? 0) * 100).toFixed(1)}%
            </span>
          </span>
        </div>
        {run?.ended_at && (
          <span className="text-(--muted)">
            {new Date(run.ended_at).toLocaleTimeString()}
          </span>
        )}
      </div>
    </div>
  );
}

const AGENT_BADGE_COLORS: Record<string, string> = {
  Researcher:  "bg-emerald-500/15 text-emerald-400 border-emerald-500/20",
  Reasoner:    "bg-indigo-500/15 text-indigo-400 border-indigo-500/20",
  Critic:      "bg-amber-500/15 text-amber-400 border-amber-500/20",
  Synthesizer: "bg-pink-500/15 text-pink-400 border-pink-500/20",
  ToolAgent:   "bg-violet-500/15 text-violet-400 border-violet-500/20",
};

const AGENT_DOT_COLORS: Record<string, string> = {
  Researcher:  "#10B981",
  Reasoner:    "#6366F1",
  Critic:      "#F59E0B",
  Synthesizer: "#EC4899",
  ToolAgent:   "#8B5CF6",
};

function AuditLogEntry({ step, index }: { step: CoTStep; index: number }) {
  // Extract agent name from "AgentName: output text" format
  const colonIdx = step.text.indexOf(":");
  const agentName = colonIdx > 0 ? step.text.slice(0, colonIdx).trim() : "Agent";
  const output = colonIdx > 0 ? step.text.slice(colonIdx + 1).trim() : step.text;
  const badgeClass = AGENT_BADGE_COLORS[agentName] ?? "bg-slate-500/15 text-slate-400 border-slate-500/20";
  const dotColor = AGENT_DOT_COLORS[agentName] ?? "#6366F1";

  const stepLabel =
    step.step_type === "premise" ? "RESEARCH" :
    step.step_type === "conclusion" ? "SYNTHESIS" :
    step.step_type === "tool_call" ? "TOOL" :
    step.step_type.toUpperCase();

  return (
    <div className="flex gap-3 px-4 py-3 border-b border-(--border) hover:bg-(--surface-raised)/50 transition-all group animate-fade-in">
      {/* Timeline rail */}
      <div className="flex flex-col items-center shrink-0">
        <div
          className="w-2.5 h-2.5 rounded-full border-2 mt-1 transition-all group-hover:scale-125"
          style={{ borderColor: dotColor, backgroundColor: dotColor + "40" }}
        />
        <div className="w-px flex-1 bg-(--border) mt-1" />
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0 pb-1">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-[10px] font-mono text-(--muted)">
            #{index + 1}
          </span>
          <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded border ${badgeClass}`}>
            {stepLabel}
          </span>
          <span className="text-[9px] font-semibold" style={{ color: dotColor }}>
            {agentName}
          </span>
        </div>
        <div className="text-xs text-(--foreground) leading-relaxed truncate">
          {output}
        </div>
        <div className="flex items-center gap-3 mt-1">
          <div className="flex items-center gap-1">
            <div className="w-12 h-[3px] rounded-full bg-(--border) overflow-hidden">
              <div
                className="h-full rounded-full transition-all duration-500"
                style={{ width: `${Math.round(step.confidence * 100)}%`, backgroundColor: dotColor }}
              />
            </div>
            <span className="text-[8px] font-mono text-(--muted)">
              {Math.round(step.confidence * 100)}%
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}

function SettingsModal({
  open,
  onClose,
  settings,
  onUpdate,
  onReset,
}: {
  open: boolean;
  onClose: () => void;
  settings: { temperature: number; maxTokens: number; streamEnabled: boolean };
  onUpdate: (p: Partial<typeof settings>) => void;
  onReset: () => void;
}) {
  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-(--background)/60 backdrop-blur-sm"
      onClick={onClose}>
      <div
        className="bg-(--surface) rounded-2xl shadow-2xl border border-(--border) w-full max-w-md mx-4 p-6 space-y-6"
        onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-bold text-(--foreground)">
            Model Settings
          </h3>
          <button
            onClick={onClose}
            className="text-(--muted) hover:text-(--foreground) text-xl font-bold">
            ×
          </button>
        </div>

        <div className="space-y-5">
          <div>
            <label className="text-xs font-semibold text-(--muted) uppercase tracking-wider">
              Temperature: {settings.temperature.toFixed(1)}
            </label>
            <input
              type="range"
              min="0"
              max="1"
              step="0.1"
              value={settings.temperature}
              onChange={(e) =>
                onUpdate({ temperature: parseFloat(e.target.value) })
              }
              className="w-full mt-2 accent-indigo-600"
            />
            <div className="flex justify-between text-[10px] text-(--muted)">
              <span>Precise</span>
              <span>Creative</span>
            </div>
          </div>
          <div>
            <label className="text-xs font-semibold text-(--muted) uppercase tracking-wider">
              Max Tokens
            </label>
            <select
              value={settings.maxTokens}
              onChange={(e) =>
                onUpdate({ maxTokens: parseInt(e.target.value) })
              }
              className="w-full mt-2 bg-(--surface-raised) border border-(--border) rounded-lg px-4 py-2.5 text-sm text-(--foreground) outline-none focus:ring-2 focus:ring-indigo-500">
              {[512, 1024, 2048, 4096].map((v) => (
                <option key={v} value={v}>
                  {v}
                </option>
              ))}
            </select>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-(--muted) uppercase tracking-wider">
              Stream Responses
            </span>
            <button
              onClick={() =>
                onUpdate({ streamEnabled: !settings.streamEnabled })
              }
              className={`w-10 h-5 rounded-full transition-colors relative ${settings.streamEnabled ? "bg-indigo-600" : "bg-slate-300"}`}>
              <span
                className={`absolute top-0.5 w-4 h-4 rounded-full bg-(--surface) shadow transition-transform ${settings.streamEnabled ? "left-5" : "left-0.5"}`}
              />
            </button>
          </div>
        </div>

        <div className="flex gap-3 pt-2">
          <button
            onClick={onReset}
            className="flex-1 py-2 text-xs font-semibold text-(--muted) border border-(--border) rounded-lg hover:bg-(--surface-raised) transition-colors">
            Reset Defaults
          </button>
          <button
            onClick={onClose}
            className="flex-1 py-2 text-xs font-semibold text-white bg-indigo-600 rounded-lg hover:bg-indigo-700 transition-colors">
            Done
          </button>
        </div>
      </div>
    </div>
  );
}

function TelemetryModal({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { kafka, cache, loading } = useTelemetry(3000, open);
  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-(--background)/60 backdrop-blur-sm"
      onClick={onClose}>
      <div
        className="bg-(--surface) rounded-2xl shadow-2xl border border-(--border) w-full max-w-2xl mx-4 p-6 space-y-6"
        onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-bold text-(--foreground) flex items-center gap-2">
            <Activity className="text-indigo-500" size={20} /> Infrastructure
            Telemetry
          </h3>
          <button
            onClick={onClose}
            className="text-(--muted) hover:text-(--foreground) text-xl font-bold">
            ×
          </button>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="bg-(--surface-raised) p-4 rounded-xl border border-(--border)">
            <h4 className="text-xs font-bold text-(--muted) uppercase tracking-widest mb-3 flex items-center justify-between">
              Event Bus (Kafka)
              {kafka?.kafka_enabled ? (
                <span className="flex items-center gap-1 text-emerald-500">
                  <CheckCircle2 size={12} /> Online
                </span>
              ) : (
                <span className="flex items-center gap-1 text-slate-500">
                  <X size={12} /> Disabled
                </span>
              )}
            </h4>
            {kafka?.topics && (
              <div className="space-y-2 mt-4">
                <p className="text-[10px] text-slate-500 mb-2 uppercase tracking-wider">
                  Active Topics
                </p>
                {Object.entries(kafka.topics).map(([k, v]) => (
                  <div
                    key={k}
                    className="flex justify-between items-center text-xs">
                    <span className="text-(--muted) capitalize">{k}</span>
                    <span className="text-indigo-400 font-mono bg-indigo-500/10 px-2 py-0.5 rounded border border-indigo-500/20">
                      {v}
                    </span>
                  </div>
                ))}
              </div>
            )}
            {!kafka && loading && (
              <div className="h-20 flex items-center justify-center">
                <div className="w-4 h-4 rounded-full border-2 border-indigo-500 border-t-transparent animate-spin"></div>
              </div>
            )}
          </div>

          <div className="bg-(--surface-raised) p-4 rounded-xl border border-(--border)">
            <h4 className="text-xs font-bold text-(--muted) uppercase tracking-widest mb-3 flex items-center justify-between">
              Trace Cache (Redis)
              {cache?.cache_enabled ? (
                <span className="flex items-center gap-1 text-emerald-500">
                  <CheckCircle2 size={12} /> Online
                </span>
              ) : (
                <span className="flex items-center gap-1 text-slate-500">
                  <X size={12} /> Disabled
                </span>
              )}
            </h4>
            {cache?.key_prefix && (
              <div className="space-y-3 mt-4">
                <div className="flex justify-between items-center text-xs">
                  <span className="text-(--muted)">Prefix</span>
                  <span className="text-indigo-400 font-mono bg-indigo-500/10 px-2 py-0.5 rounded border border-indigo-500/20">
                    {cache.key_prefix}
                  </span>
                </div>
                <p className="text-[10px] text-slate-500 mt-2 leading-relaxed border-t border-(--border) pt-2">
                  {cache.note}
                </p>
              </div>
            )}
            {!cache && loading && (
              <div className="h-20 flex items-center justify-center">
                <div className="w-4 h-4 rounded-full border-2 border-indigo-500 border-t-transparent animate-spin"></div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

export default function Home() {
  const [isConfigured, setIsConfigured] = useState(true);
  const { theme, toggleTheme } = useTheme();
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [showTelemetry, setShowTelemetry] = useState(false);
  const [showSearch, setShowSearch] = useState(false);

  const toggleSidebar = () => setIsSidebarOpen(!isSidebarOpen);

  const clock = useLiveClock();
  const health = useHealthCheck();
  const sessionNum = useSessionCounter();
  const { history, addEntry, clearHistory, searchTerm, setSearchTerm } =
    useQueryHistory();
  const {
    settings: modelSettings,
    updateSettings,
    resetSettings,
  } = useModelSettings();

  useEffect(() => {
    const hasConfig =
      !!process.env.NEXT_PUBLIC_FIREBASE_API_KEY &&
      !!process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID;
    setIsConfigured(hasConfig);
  }, []);

  const [user, setUser] = useState<User | null>(null);
  const [authLoading, setAuthLoading] = useState(true);
  const [query, setQuery] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [steps, setSteps] = useState<CoTStep[]>([]);
  const [answer, setAnswer] = useState<string | null>(null);
  const [cacheHit, setCacheHit] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [trace, setTrace] = useState<ReasoningTrace | null>(null);
  const [plan, setPlan] = useState<TaskPlan | null>(null);
  const [agentRuns, setAgentRuns] = useState<AgentRun[]>([]);
  const [delegations, setDelegations] = useState<Delegation[]>([]);
  const [thoughts, setThoughts] = useState<
    { id: string; name: string; thought: string }[]
  >([]);
  const [activeAgentId, setActiveAgentId] = useState<string | null>(null);
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [attentions, setAttentions] = useState<any[]>([]);
  const [activations, setActivations] = useState<any[]>([]);

  const abortRef = useRef<AbortController | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  const resetRun = useCallback(() => {
    setSteps([]);
    setAnswer(null);
    setCacheHit(null);
    setError(null);
    setTrace(null);
    setPlan(null);
    setAgentRuns([]);
    setDelegations([]);
    setThoughts([]);
    setActiveAgentId(null);
    setSelectedTaskId(null);
    setAttentions([]);
    setActivations([]);
  }, []);

  useEffect(() => {
    const unsub = onAuthChange((u) => {
      setUser(u);
      setAuthLoading(false);
    });
    return unsub;
  }, []);

  const handleStream = useCallback(async () => {
    if (!query.trim()) return;
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    setStreaming(true);
    resetRun();
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
              const thoughtData = data as {
                id: string;
                name: string;
                thought: string;
              };
              setThoughts((prev) => [...prev, thoughtData]);
              break;
            }
            case "agent_done": {
              const run = data as AgentRun;
              setAgentRuns((prev) => [...prev, run]);
              setActiveAgentId((curr) => (curr === run.id ? null : curr));
              if (run.status === "failed") {
                // Show as a warning thought instead of a blocking error —
                // the orchestrator still produces a final answer via stubs.
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
            case "cot_step":
              setSteps((prev) => [...prev, data as CoTStep]);
              break;
            case "attention":
              setAttentions((prev) => [...prev, data]);
              break;
            case "activation":
              setActivations((prev) => [...prev, data]);
              break;
            case "done": {
              const ans = (data as { answer: string }).answer;
              setAnswer(ans);
              setActiveAgentId(null);
              addEntry(query, ans);
              break;
            }
          }
        },
        ctrl.signal,
        modelSettings,
      );
    } catch (err: unknown) {
      if (err instanceof Error && err.name !== "AbortError")
        setError(err.message);
    } finally {
      setStreaming(false);
    }
  }, [query, resetRun, addEntry, modelSettings]);

  if (!isConfigured) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-(--background) p-4 text-center">
        <div className="bg-(--surface) p-8 sm:p-12 rounded-2xl shadow-xl border border-red-100 max-w-lg w-full">
          <div className="bg-red-50 text-red-600 w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-6">
            <Activity size={32} />
          </div>
          <h2 className="text-2xl font-bold text-(--foreground) mb-2">
            Configuration Missing
          </h2>
          <p className="text-(--muted) mb-8">
            Please set your Firebase and Gemini API keys in .env.local to
            proceed.
          </p>
          <div className="text-xs font-mono bg-(--surface-raised) p-4 rounded-lg text-slate-600 text-left space-y-2">
            <div>• NEXT_PUBLIC_FIREBASE_API_KEY</div>
            <div>• NEXT_PUBLIC_FIREBASE_PROJECT_ID</div>
            <div>• FIREBASE_PROJECT_ID (Backend)</div>
          </div>
        </div>
      </div>
    );
  }

  if (authLoading) return null;
  if (!user) return <AuthForm onAuth={() => {}} />;

  return (
    <div className="h-screen flex bg-(--background) text-(--foreground) font-sans overflow-hidden relative">
      {/* ── Mobile Sidebar Overlay ── */}
      {isSidebarOpen && (
        <div
          className="fixed inset-0 bg-(--background)/60 z-30 lg:hidden backdrop-blur-sm"
          onClick={toggleSidebar}
        />
      )}

      {/* ── Left Sidebar ── */}
      <aside
        className={`
        fixed inset-y-0 left-0 w-[280px] bg-(--surface) flex flex-col shadow-2xl z-40 transition-transform duration-300 lg:relative lg:translate-x-0
        ${isSidebarOpen ? "translate-x-0" : "-translate-x-full"}
      `}>
        <div className="p-6 flex items-center justify-between border-b border-(--border)">
          <div className="flex items-center gap-3">
            <div className="bg-indigo-600 p-2 rounded-lg">
              <Zap className="text-white" size={20} />
            </div>
            <span
              className="font-bold text-white tracking-tight text-lg uppercase"
              style={{
                fontFamily: "var(--font-display)",
                letterSpacing: "-0.02em",
              }}>
              Logic Flow
            </span>
          </div>
          <button
            className="lg:hidden text-(--muted) hover:text-white"
            onClick={toggleSidebar}>
            <X size={20} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-4 py-6 space-y-8">
          {/* Query History */}
          <div>
            <div className="flex items-center justify-between px-2 mb-4">
              <span className="text-[10px] font-bold text-(--muted) uppercase tracking-widest">
                Query History
              </span>
              <div className="flex items-center gap-2">
                {history.length > 0 && (
                  <button
                    onClick={clearHistory}
                    className="text-(--muted) hover:text-red-400 transition-colors text-[10px]"
                    title="Clear history">
                    Clear
                  </button>
                )}
                <button
                  onClick={() => setShowSearch(!showSearch)}
                  className="text-(--muted) hover:text-white transition-colors">
                  <Search size={14} />
                </button>
              </div>
            </div>
            {showSearch && (
              <input
                type="text"
                placeholder="Search history..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="w-full mb-3 bg-(--surface-raised) border border-slate-700 rounded-lg px-3 py-2 text-xs text-slate-300 outline-none focus:ring-1 focus:ring-indigo-500 placeholder:text-slate-600"
                autoFocus
              />
            )}
            <div className="space-y-1 max-h-[240px] overflow-y-auto">
              {history.length === 0 ? (
                <p className="text-[11px] text-slate-600 italic px-3 py-4 text-center">
                  No queries yet. Start by entering a question below.
                </p>
              ) : (
                history.map((entry, i) => (
                  <button
                    key={entry.id}
                    onClick={() => {
                      setQuery(entry.query);
                      setIsSidebarOpen(false);
                    }}
                    title={entry.query}
                    className={`w-full text-left px-3 py-2.5 rounded-lg text-sm transition-all flex items-center gap-3 ${i === 0 ? "bg-indigo-600/20 text-indigo-400 border border-indigo-600/30" : "text-(--muted) hover:bg-(--surface-raised) hover:text-(--foreground)"}`}>
                    <History size={14} className="shrink-0" />
                    <div className="min-w-0">
                      <span className="truncate block">
                        {entry.query.length > 35
                          ? entry.query.slice(0, 35) + "..."
                          : entry.query}
                      </span>
                      <span className="text-[10px] text-slate-600 block">
                        {new Date(entry.timestamp).toLocaleString(undefined, {
                          month: "short",
                          day: "numeric",
                          hour: "2-digit",
                          minute: "2-digit",
                        })}
                      </span>
                    </div>
                  </button>
                ))
              )}
            </div>
          </div>

          {/* Model Settings */}
          <div>
            <div className="flex items-center justify-between px-2 mb-4">
              <span className="text-[10px] font-bold text-(--muted) uppercase tracking-widest">
                Model Settings
              </span>
              <button
                onClick={() => setShowSettings(true)}
                className="text-(--muted) hover:text-white transition-colors">
                <Settings size={14} />
              </button>
            </div>
            <div className="space-y-4 px-2">
              <div className="flex items-center justify-between group">
                <span className="text-xs text-(--muted)">Temperature</span>
                <span className="text-xs text-indigo-400 font-mono">
                  {modelSettings.temperature.toFixed(1)}
                </span>
              </div>
              <div className="flex items-center justify-between group">
                <span className="text-xs text-(--muted)">Max Tokens</span>
                <span className="text-xs text-indigo-400 font-mono">
                  {modelSettings.maxTokens}
                </span>
              </div>
              <div className="flex items-center justify-between group">
                <span className="text-xs text-(--muted)">Streaming</span>
                <span
                  className={`text-xs font-mono ${modelSettings.streamEnabled ? "text-emerald-400" : "text-slate-600"}`}>
                  {modelSettings.streamEnabled ? "ON" : "OFF"}
                </span>
              </div>
              <button
                onClick={() => setShowSettings(true)}
                className="w-full text-center text-[10px] text-(--muted) hover:text-indigo-400 transition-colors py-1 border border-(--border) rounded-lg hover:border-indigo-600/30">
                Configure →
              </button>
            </div>
          </div>

          {/* Infrastructure */}
          <div className="mt-6">
            <div className="flex items-center justify-between px-2 mb-4">
              <span className="text-[10px] font-bold text-(--muted) uppercase tracking-widest">
                Infrastructure
              </span>
            </div>
            <div className="px-2">
              <button
                onClick={() => setShowTelemetry(true)}
                className="w-full flex items-center justify-center gap-2 text-[10px] text-(--muted) hover:text-indigo-400 transition-colors py-2.5 border border-(--border) rounded-lg hover:border-indigo-600/30 bg-(--surface-raised)/50 shadow-sm">
                <Server size={12} />
                View Live Telemetry
              </button>
            </div>
          </div>
        </div>

        {/* Sidebar Footer */}
        <div className="p-4 border-t border-(--border) space-y-4">
          <button
            onClick={toggleTheme}
            className="w-full flex items-center justify-between p-2 rounded-lg bg-(--surface-raised) hover:bg-(--surface-raised) transition-colors text-(--muted)">
            <span className="text-xs">Switch Theme</span>
            {theme === "dark" ? <Sun size={14} /> : <Moon size={14} />}
          </button>
          <button
            onClick={() => signOut()}
            className="w-full flex items-center gap-3 px-3 py-2 text-xs text-(--muted) hover:text-red-400 transition-colors">
            <LogOut size={14} />
            <span>Terminate Session</span>
          </button>
        </div>
      </aside>

      {/* ── Main Content ── */}
      <div className="flex-1 flex flex-col relative min-w-0">
        {/* Top Header */}
        <header className="h-16 bg-(--surface) border-b border-(--border) flex items-center justify-between px-4 sm:px-8 z-10 shadow-sm shrink-0">
          <div className="flex items-center gap-4">
            <button
              className="lg:hidden text-slate-600 p-1"
              onClick={toggleSidebar}>
              <Menu size={24} />
            </button>
          </div>

          <div className="flex items-center gap-4">
            {health.version && (
              <span className="text-[10px] text-(--muted) font-mono hidden md:inline">
                v{health.version}
              </span>
            )}
            <div className="flex items-center gap-2 text-[10px] sm:text-xs whitespace-nowrap">
              <div
                className={`h-2 w-2 rounded-full shrink-0 ${health.ok ? "bg-green-500 animate-pulse" : "bg-red-500"}`}></div>
              <span
                className={`font-medium hidden sm:inline ${health.ok ? "text-(--foreground)" : "text-red-600"}`}>
                {health.label}
              </span>
              <span className="text-(--muted)">{clock}</span>
            </div>
          </div>
        </header>

        {/* Workbench Grid */}
        <main className="flex-1 overflow-y-auto p-4 sm:p-6 space-y-4 bg-(--background)">
          {/* Row 1: Query Input Bar */}
          <section className="bg-(--surface) rounded-xl shadow-sm border border-(--border) overflow-hidden">
            <div className="p-4 flex gap-3 items-start">
              <textarea
                className="flex-1 bg-(--surface-raised) border border-(--border) rounded-xl p-3 text-sm leading-relaxed text-(--foreground) outline-none focus:ring-2 focus:ring-indigo-500/20 transition-all resize-none font-sans min-h-[60px] max-h-[100px]"
                placeholder="Enter your query here... (⌘/Ctrl+Enter to run)"
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
              <button
                onClick={handleStream}
                disabled={streaming || !query.trim()}
                className="bg-indigo-600 text-white px-5 py-3 rounded-lg text-xs font-bold hover:bg-indigo-700 shadow-lg shadow-indigo-500/20 transition-all disabled:opacity-50 whitespace-nowrap shrink-0">
                {streaming ? "Processing..." : "Run Inference"}
              </button>
            </div>
            {error && (
              <div className="mx-4 mb-3 text-xs p-3 bg-red-500/10 border border-red-500/20 text-(--error) rounded-lg animate-fade-in">
                {error}
              </div>
            )}
          </section>

          {/* Row 2: Process Topology + Final Answer */}
          <div className="grid grid-cols-1 lg:grid-cols-5 gap-4">
            {/* Process Topology */}
            <section className="lg:col-span-3 bg-(--surface) rounded-xl shadow-sm border border-(--border) overflow-hidden flex flex-col">
              <div className="px-4 py-3 border-b border-(--border) bg-(--surface-raised)/30">
                <h2 className="text-xs font-bold text-(--foreground) tracking-tight uppercase">
                  Process Topology
                </h2>
              </div>
              <div className="p-4 bg-(--surface) min-h-[200px] h-[250px] flex-1 flex flex-col relative">
                {plan ? (
                  <div className="absolute inset-0 p-4">
                    <AgentGraph
                      plan={plan}
                      runs={agentRuns}
                      delegations={delegations}
                      activeId={activeAgentId}
                      onNodeClick={setSelectedTaskId}
                    />
                  </div>
                ) : streaming ? (
                  <div className="w-full h-full flex items-center justify-center gap-8 p-4">
                    <Skeleton className="w-32 h-16 rounded-2xl" />
                    <Skeleton className="w-32 h-16 rounded-2xl" />
                    <Skeleton className="w-32 h-16 rounded-2xl" />
                  </div>
                ) : (
                  <div className="h-full flex flex-col items-center justify-center text-center space-y-3">
                    <div className="bg-(--surface-raised) p-3 rounded-2xl border border-(--border)">
                      <LayoutGrid className="text-slate-300" size={32} />
                    </div>
                    <div>
                      <p className="text-xs font-semibold text-(--foreground)">
                        No active topology
                      </p>
                      <p className="text-[10px] text-(--muted) mt-1">
                        Enter a query above to begin
                      </p>
                    </div>
                  </div>
                )}
              </div>
            </section>

            {/* Final Answer */}
            <section className="lg:col-span-2 bg-(--surface) rounded-xl shadow-sm border border-(--border) overflow-hidden flex flex-col">
              <div className="px-4 py-3 border-b border-(--border) bg-(--surface-raised)/30 flex items-center justify-between">
                <h2 className="text-xs font-bold text-(--foreground) tracking-tight uppercase">
                  Final Answer
                </h2>
                {cacheHit !== null && (
                  <span
                    className={`text-[9px] font-mono px-2 py-0.5 rounded-full ${cacheHit ? "bg-(--success)/15 text-(--success)" : "bg-(--accent)/15 text-(--accent)"}`}>
                    X-Cache: {cacheHit ? "HIT" : "MISS"}
                  </span>
                )}
              </div>
              <div className="flex-1 p-4 overflow-y-auto">
                {answer ? (() => {
                  // Try to detect and format JSON answers from the LLM
                  let parsed: any = null;
                  try {
                    const trimmed = answer.trim();
                    if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
                      parsed = JSON.parse(trimmed);
                    }
                  } catch { /* not JSON, render as plain text */ }

                  if (parsed && typeof parsed === "object") {
                    return (
                      <div className="animate-fade-in space-y-3">
                        {Object.entries(parsed).map(([key, val]) => {
                          const label = key.replace(/_/g, " ").replace(/\b\w/g, c => c.toUpperCase());
                          if (Array.isArray(val)) {
                            return (
                              <div key={key}>
                                <div className="text-[10px] font-bold text-(--accent) uppercase tracking-wider mb-1.5">
                                  {label}
                                </div>
                                <div className="space-y-2 pl-1">
                                  {(val as any[]).map((item, i) => (
                                    <div key={i} className="text-sm text-(--foreground) leading-relaxed border-l-2 border-(--accent)/30 pl-3">
                                      {typeof item === "object" ? (
                                        <div>
                                          {item.type && <span className="font-semibold text-(--accent)">{item.type}: </span>}
                                          <span>{item.description || item.text || JSON.stringify(item)}</span>
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
                              <div className="text-[10px] font-bold text-(--accent) uppercase tracking-wider mb-0.5">
                                {label}
                              </div>
                              <p className="text-sm text-(--foreground) leading-relaxed">
                                {String(val)}
                              </p>
                            </div>
                          );
                        })}
                      </div>
                    );
                  }

                  return (
                    <div className="animate-fade-in">
                      <p className="text-sm text-(--foreground) leading-relaxed whitespace-pre-wrap">
                        {answer}
                      </p>
                    </div>
                  );
                })() : streaming ? (
                  <div className="space-y-3 p-2">
                    <Skeleton className="h-4 w-full" />
                    <Skeleton className="h-4 w-[92%]" />
                    <Skeleton className="h-4 w-[75%]" />
                    <Skeleton className="h-4 w-[85%]" />
                    <Skeleton className="h-4 w-[60%]" />
                  </div>
                ) : (
                  <div className="h-full min-h-[100px] flex items-center justify-center text-(--muted) text-xs font-mono italic">
                    Answer will appear here...
                  </div>
                )}
              </div>
            </section>
          </div>

          {/* Row 3: Audit Log + Internal Thoughts */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 pb-4">
            {/* Technical Audit Log */}
            <section className="bg-(--surface) rounded-xl shadow-sm border border-(--border) flex flex-col max-h-[300px]">
              <div className="px-4 py-3 border-b border-(--border) bg-(--surface-raised)/30">
                <h2 className="text-xs font-bold text-(--foreground) tracking-tight uppercase">
                  Technical Audit Log
                </h2>
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
                  <div className="h-full min-h-[100px] flex items-center justify-center text-(--muted) text-xs font-mono italic">
                    Waiting for execution sequence...
                  </div>
                )}
              </div>
            </section>

            {/* Internal Thoughts */}
            <section className="bg-(--surface) rounded-xl shadow-sm border border-(--border) flex flex-col max-h-[300px]">
              <div className="px-4 py-3 border-b border-(--border) bg-(--surface-raised)/30 flex items-center justify-between">
                <h2 className="text-xs font-bold text-(--foreground) tracking-tight uppercase">
                  Internal Thoughts
                </h2>
                {thoughts.length > 0 && (
                  <span className="text-[9px] font-mono text-(--muted) bg-(--surface-raised) px-2 py-0.5 rounded-full border border-(--border)">
                    {thoughts.length} steps
                  </span>
                )}
              </div>
              <div className="flex-1 overflow-y-auto p-3">
                {thoughts.length > 0 ? (
                  <div className="space-y-2">
                    {thoughts.map((t, i) => {
                      const dotColor = AGENT_DOT_COLORS[t.name] ?? "#6366F1";
                      const badgeClass = AGENT_BADGE_COLORS[t.name] ?? "bg-slate-500/15 text-slate-400 border-slate-500/20";
                      const isWarning = t.thought.includes("could not reach") || t.thought.includes("timed out") || t.thought.includes("fallback");
                      const isError = t.thought.includes("error") || t.thought.includes("failed");

                      return (
                        <div
                          key={i}
                          className={`text-xs rounded-lg border animate-fade-in transition-all ${
                            isWarning
                              ? "bg-amber-500/5 border-amber-500/20"
                              : isError
                              ? "bg-red-500/5 border-red-500/20"
                              : "bg-(--surface-raised) border-(--border)"
                          }`}
                        >
                          <div className="flex items-center gap-2 px-3 pt-2.5 pb-1">
                            <div
                              className="w-1.5 h-1.5 rounded-full shrink-0"
                              style={{ backgroundColor: dotColor }}
                            />
                            <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded border ${badgeClass}`}>
                              STEP {i + 1}
                            </span>
                            <span className="text-[10px] font-semibold" style={{ color: dotColor }}>
                              {t.name}
                            </span>
                            {isWarning && <span className="text-[9px]">⚠</span>}
                          </div>
                          <div className="px-3 pb-2.5 pt-1 text-(--muted) leading-relaxed">
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
                  <div className="h-full min-h-[100px] flex flex-col items-center justify-center text-center space-y-2">
                    <Brain className="text-(--border) opacity-50" size={24} />
                    <span className="text-(--muted) text-[10px] font-mono italic">
                      Agent reasoning will appear here…
                    </span>
                  </div>
                )}
              </div>
            </section>
          </div>
        </main>
      </div>
      <AgentDetailPanel
        taskId={selectedTaskId}
        runs={agentRuns}
        plan={plan}
        streaming={streaming}
        onClose={() => setSelectedTaskId(null)}
      />

      <SettingsModal
        open={showSettings}
        onClose={() => setShowSettings(false)}
        settings={modelSettings}
        onUpdate={updateSettings}
        onReset={resetSettings}
      />

      <TelemetryModal
        open={showTelemetry}
        onClose={() => setShowTelemetry(false)}
      />
    </div>
  );
}

