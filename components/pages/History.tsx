"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { collection, query, orderBy, onSnapshot, limit, deleteDoc, doc } from "firebase/firestore";
import { db } from "@/lib/firebase";
import { useCurrentUser } from "@/components/RequireAuth";
import {
  History as HistoryIcon,
  Search,
  ChevronRight,
  Trash2,
  Loader2,
  Clock,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface TraceDoc {
  id: string;
  query: string;
  answer?: string;
  cache_hit?: boolean;
  duration_ms?: number;
  agents?: { id: string }[];
  created_at?: { toDate: () => Date } | null;
}

export default function History() {
  const user = useCurrentUser();
  const router = useRouter();
  const [traces, setTraces] = useState<TraceDoc[] | null>(null);
  const [filter, setFilter] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!user) return;
    const q = query(
      collection(db, "users", user.uid, "traces"),
      orderBy("created_at", "desc"),
      limit(100)
    );
    const unsub = onSnapshot(
      q,
      (snap) => {
        setTraces(snap.docs.map((d) => ({ id: d.id, ...(d.data() as Omit<TraceDoc, "id">) })));
      },
      (err) => setError(err.message)
    );
    return unsub;
  }, [user]);

  const filtered =
    traces?.filter((t) => t.query?.toLowerCase().includes(filter.toLowerCase())) ?? [];

  async function handleDelete(id: string, e: React.MouseEvent) {
    e.stopPropagation();
    if (!user) return;
    if (!confirm("Delete this trace?")) return;
    await deleteDoc(doc(db, "users", user.uid, "traces", id));
  }

  return (
    <div className="p-4 md:p-8 max-w-5xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-display font-semibold tracking-tight flex items-center gap-2">
            <HistoryIcon size={22} className="text-primary" />
            Trace History
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Saved reasoning runs from your workbench sessions.
          </p>
        </div>
        <Button onClick={() => router.push("/")} variant="outline" size="sm">
          New run
        </Button>
      </div>

      <div className="relative">
        <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Search by prompt…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="pl-10 bg-surface"
        />
      </div>

      {error && (
        <div className="text-xs p-3 rounded-md bg-destructive/10 text-destructive border border-destructive/25">
          {error}
        </div>
      )}

      {traces === null ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 className="animate-spin text-muted-foreground" size={20} />
        </div>
      ) : filtered.length === 0 ? (
        <div className="bg-surface border border-border rounded-2xl p-16 text-center shadow-elegant">
          <HistoryIcon size={32} className="mx-auto mb-4 text-muted-foreground opacity-50" />
          <h3 className="text-sm font-semibold mb-1">
            {filter ? "No matching traces" : "No traces yet"}
          </h3>
          <p className="text-xs text-muted-foreground mb-6">
            {filter
              ? "Try a different search term."
              : "Run an inference on the workbench to start building history."}
          </p>
          {!filter && (
            <Button onClick={() => router.push("/")} className="bg-gradient-primary text-primary-foreground border-0">
              Open Workbench
            </Button>
          )}
        </div>
      ) : (
        <div className="bg-surface border border-border rounded-2xl shadow-elegant overflow-hidden divide-y divide-border">
          {filtered.map((t) => (
            <button
              key={t.id}
              onClick={() => router.push(`/inspector/${t.id}`)}
              className="w-full text-left px-4 py-4 flex items-center gap-4 hover:bg-surface-raised/50 transition-colors group"
            >
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium text-foreground truncate">{t.query}</div>
                <div className="flex items-center gap-3 mt-1 text-[11px] text-muted-foreground">
                  {t.created_at?.toDate && (
                    <span className="flex items-center gap-1">
                      <Clock size={11} />
                      {t.created_at.toDate().toLocaleString()}
                    </span>
                  )}
                  {t.agents && (
                    <span className="font-mono">{t.agents.length} agents</span>
                  )}
                  {typeof t.duration_ms === "number" && (
                    <span className="font-mono">{(t.duration_ms / 1000).toFixed(1)}s</span>
                  )}
                  {t.cache_hit !== undefined && (
                    <span
                      className={cn(
                        "font-mono px-1.5 py-0.5 rounded border text-[10px]",
                        t.cache_hit
                          ? "text-success border-success/30 bg-success/5"
                          : "text-primary border-primary/30 bg-primary/5"
                      )}
                    >
                      {t.cache_hit ? "HIT" : "MISS"}
                    </span>
                  )}
                </div>
              </div>
              <button
                onClick={(e) => handleDelete(t.id, e)}
                className="p-2 rounded-md text-muted-foreground hover:text-destructive hover:bg-destructive/10 opacity-0 group-hover:opacity-100 transition-all"
                aria-label="Delete trace"
              >
                <Trash2 size={14} />
              </button>
              <ChevronRight size={16} className="text-muted-foreground" />
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
