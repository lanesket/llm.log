// Status
export interface StatusResponse {
  proxy_running: boolean;
  pid: number | null;
  uptime_seconds: number | null;
  port: number | null;
}

// Dashboard
export interface Totals {
  requests: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  total_cost: number;
  unknown_cost_requests: number;
  errors: number;
  avg_duration_ms: number;
}

export interface BreakdownRow {
  name: string;
  provider?: string;
  requests: number;
  total_cost: number;
  tokens: number;
}

export interface ChartPoint {
  timestamp: string;
  requests: number;
  cost: number;
  tokens: number;
}

export interface DashboardResponse {
  totals: Totals;
  prev_totals: Totals | null;
  by_provider: BreakdownRow[];
  by_model: BreakdownRow[];
  by_source: BreakdownRow[];
  chart: ChartPoint[];
}

// Requests
export interface RequestItem {
  id: number;
  timestamp: string;
  provider: string;
  model: string;
  endpoint: string;
  source: string;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  total_cost: number | null;
  duration_ms: number;
  streaming: boolean;
  status_code: number;
}

export interface RequestsResponse {
  items: RequestItem[];
  total: number | null;
  next_cursor: string;
}

export interface RequestDetailResponse extends RequestItem {
  request_body: string;
  response_body: string;
}

// Analytics
export interface AnalyticsPoint {
  timestamp: string;
  requests: number;
  cost: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
}

export interface ProviderTimePoint {
  timestamp: string;
  provider: string;
  requests: number;
  cost: number;
}

export interface ModelStat {
  model: string;
  provider: string;
  requests: number;
  cost: number;
  avg_duration_ms: number;
}

export interface ExpensiveRequest {
  id: number;
  timestamp: string;
  model: string;
  cost: number;
  total_tokens: number;
}

export interface CostDistribution {
  p50: number;
  p90: number;
  p95: number;
  p99: number;
  max: number;
}

export interface HeatmapEntry {
  day_of_week: number;
  hour: number;
  requests: number;
  cost: number;
}

export interface CumulativePoint {
  timestamp: string;
  cumulative: number;
}

export interface CacheHitPoint {
  timestamp: string;
  rate: number;
}

export interface AvgTokensRow {
  model: string;
  avg_input: number;
  avg_output: number;
}

export interface AnalyticsResponse {
  over_time: AnalyticsPoint[];
  by_provider_over_time: ProviderTimePoint[];
  top_models: ModelStat[];
  top_expensive: ExpensiveRequest[];
  cost_distribution: CostDistribution;
  cumulative_cost: CumulativePoint[];
  cache_hit_rate: CacheHitPoint[];
  avg_tokens_per_request: AvgTokensRow[];
  heatmap: HeatmapEntry[];
}

// Filters
export interface FiltersResponse {
  providers: string[];
  models: string[];
  sources: string[];
}

// Proxy control
export interface ProxyActionResponse {
  ok: boolean;
  pid?: number;
  port?: number;
}

// Request params
export interface RequestsParams {
  cursor?: string;
  limit?: number;
  sort?: string;
  dir?: string;
  provider?: string;
  model?: string;
  source?: string;
  status_code?: number;
  streaming?: boolean;
  min_cost?: number;
  max_cost?: number;
  min_tokens?: number;
  max_tokens?: number;
  from?: string;
  to?: string;
  search?: string;
}

export interface AnalyticsParams {
  from?: string;
  to?: string;
  granularity?: string;
}
