"use client";

import { useEffect, useState } from "react";
import { useModelSettings, useTheme } from "@/lib/hooks";
import { useCurrentUser } from "@/components/RequireAuth";
import { signOut } from "@/lib/firebase";
import {
  getApiBaseUrl,
  setApiBaseUrl,
  clearApiBaseUrl,
  getEnvDefault,
} from "@/lib/config";
import { Settings as SettingsIcon, RotateCcw, LogOut, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import { toast } from "sonner";

const AGENTS = ["Researcher", "Reasoner", "Critic", "Synthesizer", "ToolAgent"];
const AGENT_TOGGLE_KEY = "cot_agent_toggles";

export default function Settings() {
  const { settings, updateSettings, resetSettings } = useModelSettings();
  const { theme, toggleTheme } = useTheme();
  const user = useCurrentUser();

  const [apiUrl, setApiUrl] = useState("");
  const [agentToggles, setAgentToggles] = useState<Record<string, boolean>>({});

  useEffect(() => {
    setApiUrl(getApiBaseUrl());
    try {
      const raw = localStorage.getItem(AGENT_TOGGLE_KEY);
      setAgentToggles(raw ? JSON.parse(raw) : Object.fromEntries(AGENTS.map((a) => [a, true])));
    } catch {
      setAgentToggles(Object.fromEntries(AGENTS.map((a) => [a, true])));
    }
  }, []);

  function saveApiUrl() {
    setApiBaseUrl(apiUrl);
    toast.success("Backend URL saved", { description: apiUrl });
  }
  function resetApiUrl() {
    clearApiBaseUrl();
    setApiUrl(getEnvDefault());
    toast.success("Reset to default", { description: getEnvDefault() });
  }

  function toggleAgent(agent: string) {
    const next = { ...agentToggles, [agent]: !agentToggles[agent] };
    setAgentToggles(next);
    localStorage.setItem(AGENT_TOGGLE_KEY, JSON.stringify(next));
  }

  return (
    <div className="p-3 sm:p-4 md:p-8 max-w-3xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-display font-semibold tracking-tight flex items-center gap-2">
          <SettingsIcon size={22} className="text-primary" />
          Settings
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          Tune backend connection, model behavior, and appearance.
        </p>
      </div>

      {/* Backend */}
      <Card title="Backend connection" description="Where the workbench sends inference requests.">
        <div className="space-y-3">
          <Label htmlFor="api-url" className="text-xs uppercase tracking-wider text-muted-foreground">
            API base URL
          </Label>
          <div className="flex flex-col sm:flex-row gap-2">
            <Input
              id="api-url"
              value={apiUrl}
              onChange={(e) => setApiUrl(e.target.value)}
              placeholder="http://localhost:8080"
              className="bg-surface-raised font-mono text-sm"
            />
            <Button onClick={saveApiUrl} size="sm" className="bg-primary hover:bg-primary/90 text-primary-foreground border-0 sm:w-auto">
              <Save size={14} className="mr-1" /> Save
            </Button>
            <Button onClick={resetApiUrl} variant="outline" size="sm" className="sm:w-auto">
              <RotateCcw size={14} className="mr-1" /> Reset
            </Button>
          </div>
          <p className="text-[11px] text-muted-foreground">
            Default from env: <span className="font-mono">{getEnvDefault()}</span>. Override is stored locally
            in this browser.
          </p>
        </div>
      </Card>

      {/* Model */}
      <Card title="Model" description="Settings sent with every reasoning request.">
        <div className="space-y-5">
          <div>
            <div className="flex items-center justify-between mb-2">
              <Label className="text-xs uppercase tracking-wider text-muted-foreground">
                Temperature
              </Label>
              <span className="font-mono text-sm text-primary">
                {settings.temperature.toFixed(1)}
              </span>
            </div>
            <Slider
              value={[settings.temperature]}
              min={0}
              max={1}
              step={0.1}
              onValueChange={([v]) => updateSettings({ temperature: v })}
            />
            <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
              <span>Precise</span>
              <span>Creative</span>
            </div>
          </div>

          <div>
            <Label className="text-xs uppercase tracking-wider text-muted-foreground mb-2 block">
              Max tokens
            </Label>
            <select
              value={settings.maxTokens}
              onChange={(e) => updateSettings({ maxTokens: parseInt(e.target.value) })}
              className="w-full bg-surface-raised border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary/30"
            >
              {[512, 1024, 2048, 4096].map((v) => (
                <option key={v} value={v}>
                  {v}
                </option>
              ))}
            </select>
          </div>

          <div className="flex items-center justify-between gap-4">
            <div>
              <Label className="text-sm">Stream responses</Label>
              <p className="text-[11px] text-muted-foreground mt-0.5">
                Render the reasoning trace as it arrives.
              </p>
            </div>
            <Switch
              checked={settings.streamEnabled}
              onCheckedChange={(checked) => updateSettings({ streamEnabled: checked })}
            />
          </div>

          <Button onClick={resetSettings} variant="outline" size="sm">
            <RotateCcw size={14} className="mr-1" /> Reset to defaults
          </Button>
        </div>
      </Card>

      {/* Agents */}
      <Card title="Agents" description="Enable or disable agent roles for the next run.">
        <div className="space-y-3">
          {AGENTS.map((a) => (
            <div key={a} className="flex items-center justify-between">
              <Label className="text-sm">{a}</Label>
              <Switch
                checked={agentToggles[a] !== false}
                onCheckedChange={() => toggleAgent(a)}
              />
            </div>
          ))}
          <p className="text-[11px] text-muted-foreground pt-2">
            Toggles are saved locally and surfaced to the backend on the next request.
          </p>
        </div>
      </Card>

      {/* Appearance */}
      <Card title="Appearance" description="Theme preference for this browser.">
        <div className="flex items-center justify-between">
          <Label className="text-sm">Dark theme</Label>
          <Switch checked={theme === "dark"} onCheckedChange={toggleTheme} />
        </div>
      </Card>

      {/* Account */}
      <Card title="Account" description="Signed-in user.">
        <div className="space-y-3">
          <div className="text-sm">
            <span className="text-muted-foreground">Email: </span>
            <span className="font-mono">{user?.email ?? "—"}</span>
          </div>
          <div className="text-xs text-muted-foreground">
            User ID: <span className="font-mono">{user?.uid ?? "—"}</span>
          </div>
          <Button
            onClick={() => signOut()}
            variant="outline"
            size="sm"
            className="text-destructive hover:text-destructive"
          >
            <LogOut size={14} className="mr-1" /> Sign out
          </Button>
        </div>
      </Card>
    </div>
  );
}

function Card({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="bg-surface border border-border rounded-2xl p-4 sm:p-6 shadow-elegant">
      <div className="mb-5">
        <h2 className="text-base font-semibold">{title}</h2>
        {description && <p className="text-xs text-muted-foreground mt-1">{description}</p>}
      </div>
      {children}
    </div>
  );
}
