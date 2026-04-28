"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import {
  Brain,
  History,
  Settings,
  Activity,
  LogOut,
  Moon,
  Sun,
  Plus,
  Server,
} from "lucide-react";
import { onAuthChange, signOut } from "@/lib/firebase";
import { useTheme, useLiveClock, useHealthCheck } from "@/lib/hooks";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { User } from "firebase/auth";

const NAV = [
  { to: "/", label: "Workbench", icon: Brain, end: true },
  { to: "/history", label: "History", icon: History },
  { to: "/settings", label: "Settings", icon: Settings },
];

export function AppShell({ children }: { children: React.ReactNode }) {
  const { theme, toggleTheme } = useTheme();
  const clock = useLiveClock();
  const health = useHealthCheck();
  const router = useRouter();
  const pathname = usePathname();
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => onAuthChange(setUser), []);

  return (
    <div className="h-screen flex bg-background text-foreground overflow-hidden">
      {/* Sidebar */}
      <aside className="hidden md:flex w-[240px] shrink-0 flex-col border-r border-sidebar-border bg-sidebar">
        <div className="h-16 px-5 flex items-center gap-3 border-b border-sidebar-border">
          <div className="w-8 h-8 rounded-lg bg-gradient-primary flex items-center justify-center">
            <Brain className="text-primary-foreground" size={18} />
          </div>
          <div className="flex flex-col leading-tight">
            <span className="font-display font-semibold text-sm tracking-tight text-sidebar-foreground">
              Logic Flow
            </span>
            <span className="text-[10px] uppercase tracking-widest text-muted-foreground">
              Workbench
            </span>
          </div>
        </div>

        <div className="px-3 py-4">
          <Button
            onClick={() => router.push("/")}
            className="w-full justify-start gap-2 bg-gradient-primary hover:opacity-90 text-primary-foreground border-0 shadow-elegant"
            size="sm"
          >
            <Plus size={16} /> New run
          </Button>
        </div>

        <nav className="flex-1 px-3 space-y-1">
          {NAV.map(({ to, label, icon: Icon, end }) => (
            <Link
              key={to}
              href={to}
              className={
                cn(
                  "flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors",
                  (end ? pathname === to : pathname.startsWith(to))
                    ? "bg-sidebar-accent text-sidebar-accent-foreground font-medium"
                    : "text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-foreground"
                )
              }
            >
              <Icon size={16} />
              <span>{label}</span>
            </Link>
          ))}
        </nav>

        <div className="p-3 border-t border-sidebar-border space-y-2">
          <div className="px-3 py-2 rounded-md bg-sidebar-accent/50 text-[11px] flex items-center gap-2">
            <span
              className={cn(
                "w-1.5 h-1.5 rounded-full shrink-0",
                health.ok ? "bg-success animate-pulse-dot" : "bg-destructive"
              )}
            />
            <span className="text-muted-foreground">{health.label}</span>
            {health.version && (
              <span className="ml-auto font-mono text-muted-foreground">
                v{health.version}
              </span>
            )}
          </div>
          <button
            onClick={toggleTheme}
            className="w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-foreground transition-colors"
          >
            {theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}
            <span>{theme === "dark" ? "Light theme" : "Dark theme"}</span>
          </button>
          <div className="px-3 py-2 text-[11px] text-muted-foreground border-t border-sidebar-border">
            <div className="truncate">{user?.email ?? "—"}</div>
          </div>
          <button
            onClick={() => signOut()}
            className="w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm text-muted-foreground hover:text-destructive transition-colors"
          >
            <LogOut size={16} />
            <span>Sign out</span>
          </button>
        </div>
      </aside>

      {/* Main area */}
      <div className="flex-1 flex flex-col min-w-0">
        <header className="h-16 shrink-0 flex items-center justify-between px-4 md:px-6 border-b border-border bg-surface">
          <div className="md:hidden flex items-center gap-2">
            <div className="w-7 h-7 rounded-md bg-gradient-primary flex items-center justify-center">
              <Brain className="text-primary-foreground" size={16} />
            </div>
            <span className="font-display font-semibold text-sm">Logic Flow</span>
          </div>
          <div className="ml-auto flex items-center gap-4 text-xs">
            <div className="hidden sm:flex items-center gap-2">
              <Server size={12} className="text-muted-foreground" />
              <span className="font-mono text-muted-foreground">{clock}</span>
            </div>
            <div className="flex items-center gap-2">
              <Activity size={12} className={health.ok ? "text-success" : "text-destructive"} />
              <span className="text-muted-foreground hidden sm:inline">{health.label}</span>
            </div>
          </div>
        </header>

        <main className="flex-1 overflow-y-auto bg-background">{children}</main>
      </div>
    </div>
  );
}
