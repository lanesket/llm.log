import type {
  StatusResponse,
  DashboardResponse,
  RequestsResponse,
  RequestDetailResponse,
  AnalyticsResponse,
  FiltersResponse,
  ProxyActionResponse,
  RequestsParams,
  AnalyticsParams,
} from './types';

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

async function fetchAPI<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, options);
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, body.error || res.statusText);
  }
  return res.json();
}

function buildParams(params: object): string {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') {
      sp.set(k, String(v));
    }
  }
  return sp.toString();
}

export async function fetchStatus(): Promise<StatusResponse> {
  return fetchAPI('/api/status');
}

export async function fetchDashboard(from: string, to: string): Promise<DashboardResponse> {
  const q = buildParams({ from, to });
  return fetchAPI(`/api/dashboard?${q}`);
}

export async function fetchRequests(params: RequestsParams): Promise<RequestsResponse> {
  const q = buildParams(params);
  return fetchAPI(`/api/requests?${q}`);
}

export async function fetchRequestDetail(id: number): Promise<RequestDetailResponse> {
  return fetchAPI(`/api/requests/${id}`);
}

export async function fetchAnalytics(params: AnalyticsParams): Promise<AnalyticsResponse> {
  const q = buildParams(params);
  return fetchAPI(`/api/analytics?${q}`);
}

export async function fetchFilters(from: string, to: string): Promise<FiltersResponse> {
  const q = buildParams({ from, to });
  return fetchAPI(`/api/filters?${q}`);
}

export async function proxyStart(): Promise<ProxyActionResponse> {
  return fetchAPI('/api/proxy/start', { method: 'POST' });
}

export async function proxyStop(): Promise<ProxyActionResponse> {
  return fetchAPI('/api/proxy/stop', { method: 'POST' });
}

export { ApiError };
