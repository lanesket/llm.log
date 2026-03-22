export const PROVIDER_COLORS: Record<string, string> = {
  openai: '#10b981',
  anthropic: '#f97316',
  deepseek: '#3b82f6',
  groq: '#8b5cf6',
  mistral: '#ef4444',
  together: '#eab308',
  fireworks: '#ec4899',
  openrouter: '#06b6d4',
  perplexity: '#a855f7',
  xai: '#6366f1',
};

export const FALLBACK_COLORS = ['#10b981', '#3b82f6', '#f97316', '#8b5cf6', '#ef4444', '#eab308', '#ec4899', '#06b6d4', '#a855f7', '#6366f1'];

export function getProviderColor(name: string, index: number = 0): string {
  return PROVIDER_COLORS[name.toLowerCase()] ?? FALLBACK_COLORS[index % FALLBACK_COLORS.length];
}

// Chart colors — used in recharts (which needs raw hex, not CSS vars)
export const CHART_COLORS = {
  primary: '#10b981',     // main line/area (emerald)
  input: '#3b82f6',       // input tokens (blue)
  output: '#10b981',      // output tokens (emerald)
  cacheRead: '#f59e0b',   // cache read (amber)
  cacheWrite: '#8b5cf6',  // cache write (purple)
  cost: '#10b981',        // cost charts (emerald)
  cacheHit: '#10b981',    // cache hit rate (emerald)
  latency: '#f97316',     // latency/duration (orange)
} as const;
