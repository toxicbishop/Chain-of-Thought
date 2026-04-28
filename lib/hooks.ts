"use client";

import { getKafkaStatus, getCacheStatus, KafkaStatus, CacheStatus } from "./api";
import { useState, useEffect, useCallback } from "react";

// ── Query History ──────────────────────────────────────────────────────────

export interface HistoryEntry {
  id: string;
  query: string;
  answer: string | null;
  timestamp: number;
}

const HISTORY_KEY = "cot_query_history";
const MAX_HISTORY = 50;

function loadHistory(): HistoryEntry[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = localStorage.getItem(HISTORY_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function saveHistory(entries: HistoryEntry[]) {
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(entries.slice(0, MAX_HISTORY)));
  } catch { /* quota exceeded — ignore */ }
}

export function useQueryHistory() {
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const [searchTerm, setSearchTerm] = useState("");

  useEffect(() => { setHistory(loadHistory()); }, []);

  const addEntry = useCallback((query: string, answer: string | null) => {
    const entry: HistoryEntry = {
      id: crypto.randomUUID(),
      query,
      answer,
      timestamp: Date.now(),
    };
    setHistory((prev) => {
      const next = [entry, ...prev.filter((e) => e.query !== query)].slice(0, MAX_HISTORY);
      saveHistory(next);
      return next;
    });
  }, []);

  const clearHistory = useCallback(() => {
    setHistory([]);
    localStorage.removeItem(HISTORY_KEY);
  }, []);

  const filtered = searchTerm
    ? history.filter((e) => e.query.toLowerCase().includes(searchTerm.toLowerCase()))
    : history;

  return { history: filtered, addEntry, clearHistory, searchTerm, setSearchTerm };
}

// ── Live Clock ─────────────────────────────────────────────────────────────

export function useLiveClock() {
  const [time, setTime] = useState("");
  useEffect(() => {
    const fmt = () => {
      const now = new Date();
      setTime(
        now.toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit", hour12: false, timeZone: "Asia/Kolkata" }) + " IST"
      );
    };
    fmt();
    const id = setInterval(fmt, 10_000);
    return () => clearInterval(id);
  }, []);
  return time;
}

// ── Health Check ───────────────────────────────────────────────────────────

export interface HealthStatus {
  ok: boolean;
  version: string;
  label: string;
}

export function useHealthCheck(intervalMs = 30_000) {
  const [status, setStatus] = useState<HealthStatus>({ ok: false, version: "", label: "Checking..." });

  useEffect(() => {
    let alive = true;
    const check = async () => {
      try {
        const res = await fetch("/health", { signal: AbortSignal.timeout(5000) });
        if (!alive) return;
        if (res.ok) {
          const data = await res.json();
          setStatus({ ok: true, version: data.version || "1.0.0", label: "System Online" });
        } else {
          setStatus({ ok: false, version: "", label: "Degraded" });
        }
      } catch {
        if (!alive) return;
        setStatus({ ok: false, version: "", label: "Offline" });
      }
    };
    check();
    const id = setInterval(check, intervalMs);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [intervalMs]);

  return status;
}

// ── Telemetry ──────────────────────────────────────────────────────────────

export function useTelemetry(intervalMs = 30_000, active = false) {
  const [kafka, setKafka] = useState<KafkaStatus | null>(null);
  const [cache, setCache] = useState<CacheStatus | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!active) return;
    let alive = true;
    const fetchTelemetry = async () => {
      setLoading(true);
      try {
        const [k, c] = await Promise.all([getKafkaStatus(), getCacheStatus()]);
        if (alive) {
          setKafka(k);
          setCache(c);
        }
      } catch (err) {
        console.error("Failed to fetch telemetry:", err);
      } finally {
        if (alive) setLoading(false);
      }
    };
    fetchTelemetry();
    const id = setInterval(fetchTelemetry, intervalMs);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [intervalMs, active]);

  return { kafka, cache, loading };
}

// ── Session Counter ────────────────────────────────────────────────────────

const SESSION_KEY = "cot_session_count";

export function useSessionCounter() {
  const [session, setSession] = useState(1);
  useEffect(() => {
    try {
      const prev = parseInt(localStorage.getItem(SESSION_KEY) ?? "0", 10);
      const next = prev + 1;
      localStorage.setItem(SESSION_KEY, String(next));
      setSession(next);
    } catch { /* ignore */ }
  }, []);
  return session;
}

// ── Theme Persistence ──────────────────────────────────────────────────────

const THEME_KEY = "cot_theme";

export function useTheme() {
  const [theme, setThemeState] = useState<"light" | "dark">("light");

  useEffect(() => {
    const saved = (localStorage.getItem(THEME_KEY) as "light" | "dark") ?? "light";
    setThemeState(saved);
    document.documentElement.setAttribute("data-theme", saved);
  }, []);

  const toggleTheme = useCallback(() => {
    setThemeState((prev) => {
      const next = prev === "dark" ? "light" : "dark";
      localStorage.setItem(THEME_KEY, next);
      document.documentElement.setAttribute("data-theme", next);
      return next;
    });
  }, []);

  return { theme, toggleTheme };
}

// ── Model Settings ─────────────────────────────────────────────────────────

export interface ModelSettings {
  temperature: number;
  maxTokens: number;
  streamEnabled: boolean;
}

const SETTINGS_KEY = "cot_model_settings";

const defaultSettings: ModelSettings = {
  temperature: 0.5,
  maxTokens: 2048,
  streamEnabled: true,
};

export function useModelSettings() {
  const [settings, setSettingsState] = useState<ModelSettings>(defaultSettings);

  useEffect(() => {
    try {
      const raw = localStorage.getItem(SETTINGS_KEY);
      if (raw) setSettingsState({ ...defaultSettings, ...JSON.parse(raw) });
    } catch { /* ignore */ }
  }, []);

  const updateSettings = useCallback((patch: Partial<ModelSettings>) => {
    setSettingsState((prev) => {
      const next = { ...prev, ...patch };
      localStorage.setItem(SETTINGS_KEY, JSON.stringify(next));
      return next;
    });
  }, []);

  const resetSettings = useCallback(() => {
    setSettingsState(defaultSettings);
    localStorage.removeItem(SETTINGS_KEY);
  }, []);

  return { settings, updateSettings, resetSettings };
}
