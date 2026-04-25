/**
 * Typed client for the Go CoT backend.
 *
 * All paths are relative (/api/*, /auth/*) so Next.js rewrites
 * in next.config.ts forward them to the Go server transparently.
 * Callers never need to know the backend origin.
 */

import { getIdToken } from "./firebase";

// ── Shared types (mirror of internal/transformer/types.go) ──────────────────

export interface CoTStep {
  index: number;
  step_type: "premise" | "inference" | "conclusion" | "tool_call";
  text: string;
  confidence: number;
}

export interface ToolCall {
  name: string;
  inputs: Record<string, string>;
  output: string;
}

export interface AttentionSnapshot {
  layer: number;
  head: number;
  weights: number[][];
}

export interface LayerActivation {
  layer: number;
  token_means: number[];
}

// ── Orchestrator types (mirror of internal/transformer/types.go) ────────────

export interface PlannedTask {
  id: string;
  agent: string;
  task: string;
  depends_on?: string[];
}

export interface TaskPlan {
  goal: string;
  tasks: PlannedTask[];
}

export interface AgentRun {
  id: string;
  name: string;
  role: string;
  task: string;
  thought: string;
  output: string;
  confidence: number;
  status: "done" | "failed" | "skipped";
  depends_on?: string[];
  started_at: string;
  ended_at: string;
  tool_calls?: ToolCall[];
}

export interface Delegation {
  from: string;
  to: string;
  reason: string;
  task_id?: string;
}

export interface ReasoningTrace {
  query: string;
  answer: string;
  tokens: string[];
  cot_steps: CoTStep[];
  attentions: AttentionSnapshot[];
  activations: LayerActivation[];
  tool_calls: ToolCall[];
  plan?: TaskPlan;
  agents?: AgentRun[];
  delegations?: Delegation[];
  engine?: "orchestrator" | "transformer";
}

export interface AuthMe {
  email: string;
  user_id: string;
  issued_at: string;
  expires_at: string;
}

// ── Internal fetch wrapper ───────────────────────────────────────────────────

async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = await getIdToken();

  const res = await fetch(path, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  });

  if (!res.ok) {
    const body = await res.text();
    throw new Error(`${res.status} ${res.statusText}: ${body}`);
  }

  return res.json() as Promise<T>;
}

// ── Public API ───────────────────────────────────────────────────────────────

/** GET /health — unauthenticated health check. */
export async function healthCheck(): Promise<{ status: string; version: string }> {
  const res = await fetch("/health");
  return res.json();
}

/** GET /auth/me — validate session and return token claims. */
export async function getMe(): Promise<AuthMe> {
  return apiFetch<AuthMe>("/auth/me");
}

/**
 * POST /api/reason — run a full reasoning trace (cache-aware).
 *
 * The response header `X-Cache` will be "HIT" or "MISS".
 * Use `reason()` when you want the full trace at once.
 */
export async function reason(
  query: string,
  settings?: { temperature: number; maxTokens: number }
): Promise<ReasoningTrace> {
  return apiFetch<ReasoningTrace>("/api/reason", {
    method: "POST",
    body: JSON.stringify({ query, settings }),
  });
}

/**
 * POST /api/reason/stream — SSE stream for live step-by-step animation.
 *
 * @param query      The reasoning query.
 * @param onEvent    Called for each SSE event. Return false to cancel early.
 *
 * Event types: "meta" | "plan" | "agent_start" | "agent_thought" | "agent_done" |
 *              "delegation" | "cot_step" | "tool_call" | "attention" | "activation" | "done"
 */
export async function reasonStream(
  query: string,
  onEvent: (type: string, data: unknown) => boolean | void,
  signal?: AbortSignal,
  settings?: { temperature: number; maxTokens: number }
): Promise<void> {
  const token = await getIdToken();

  const res = await fetch("/api/reason/stream", {
    method: "POST",
    signal,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ query, settings }),
  });

  if (!res.ok || !res.body) {
    throw new Error(`Stream failed: ${res.status} ${res.statusText}`);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split("\n\n");
    buffer = parts.pop() ?? "";

    for (const chunk of parts) {
      const lines = chunk.trim().split("\n");
      let eventType = "message";
      let eventData = "";

      for (const line of lines) {
        if (line.startsWith("event: ")) eventType = line.slice(7).trim();
        if (line.startsWith("data: ")) eventData = line.slice(6).trim();
      }

      if (eventData) {
        let parsed: unknown;
        try {
          parsed = JSON.parse(eventData);
        } catch {
          console.warn(`[SSE] malformed event skipped (type=${eventType}):`, eventData);
          continue;
        }
        const shouldContinue = onEvent(eventType, parsed);
        if (shouldContinue === false) {
          reader.cancel();
          return;
        }
      }
    }
  }
}

/**
 * POST /api/attention/{layer}/{head} — attention weights for a specific head.
 */
export async function getAttention(
  query: string,
  layer: number,
  head: number
): Promise<AttentionSnapshot> {
  return apiFetch<AttentionSnapshot>(`/api/attention/${layer}/${head}`, {
    method: "POST",
    body: JSON.stringify({ query }),
  });
}

/**
 * POST /api/activations — layer activation values for all tokens.
 */
export async function getActivations(
  query: string
): Promise<{ activations: LayerActivation[]; tokens: string[] }> {
  return apiFetch(`/api/activations`, {
    method: "POST",
    body: JSON.stringify({ query }),
  });
}

/** GET /api/kafka/status */
export async function kafkaStatus(): Promise<Record<string, unknown>> {
  return apiFetch("/api/kafka/status");
}

/** GET /api/cache/status */
export async function cacheStatus(): Promise<Record<string, unknown>> {
  return apiFetch("/api/cache/status");
}

/** DELETE /api/cache — invalidate cached trace for a query. */
export async function invalidateCache(query: string): Promise<{ status: string; message: string }> {
  return apiFetch<{ status: string; message: string }>("/api/cache", {
    method: "DELETE",
    body: JSON.stringify({ query }),
  });
}

// ── Telemetry API ────────────────────────────────────────────────────────────

export interface KafkaStatus {
  kafka_enabled: boolean;
  status: string;
  topics: Record<string, string>;
}

export interface CacheStatus {
  cache_enabled: boolean;
  status: string;
  key_prefix: string;
  note: string;
}

/** GET /api/kafka/status */
export async function getKafkaStatus(): Promise<KafkaStatus> {
  return apiFetch<KafkaStatus>("/api/kafka/status");
}

/** GET /api/cache/status */
export async function getCacheStatus(): Promise<CacheStatus> {
  return apiFetch<CacheStatus>("/api/cache/status");
}
