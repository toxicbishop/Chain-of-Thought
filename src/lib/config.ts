/**
 * Backend URL resolution.
 * Order of precedence:
 *   1. localStorage override (set from /settings)
 *   2. NEXT_PUBLIC_API_URL env var
 *   3. same-origin Next rewrites
 */
const STORAGE_KEY = "cot_api_base_url";
const ENV_DEFAULT = process.env.NEXT_PUBLIC_API_URL ?? "";

export function getApiBaseUrl(): string {
  if (typeof window === "undefined") return ENV_DEFAULT;
  return localStorage.getItem(STORAGE_KEY) || ENV_DEFAULT;
}

export function setApiBaseUrl(url: string) {
  if (url.trim()) localStorage.setItem(STORAGE_KEY, url.trim());
  else localStorage.removeItem(STORAGE_KEY);
}

export function clearApiBaseUrl() {
  localStorage.removeItem(STORAGE_KEY);
}

export function getEnvDefault() {
  return ENV_DEFAULT;
}

/** Build an absolute backend URL from a relative path like "/api/reason". */
export function apiUrl(path: string): string {
  const base = getApiBaseUrl().replace(/\/+$/, "");
  if (!base) return path.startsWith("/") ? path : "/" + path;
  return path.startsWith("/") ? base + path : base + "/" + path;
}
