import { useMemo, Fragment, useCallback } from 'react';
import { Loader2Icon } from 'lucide-react';
import { usePolling } from '@/hooks/usePolling';
import { useTimeRange } from '@/hooks/useTimeRange';
import { fetchAnalytics } from '@/lib/api';
import { DateRangePicker } from '@/components/DateRangePicker';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { formatCost, formatTokens, formatDate, formatDuration } from '@/lib/utils';
import { getProviderColor, CHART_COLORS } from '@/lib/constants';
import { EmptyState } from '@/components/EmptyState';
import { SectionHeading } from '@/components/SectionHeading';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Link } from 'react-router-dom';
import type { HeatmapEntry, ProviderTimePoint } from '@/lib/types';
import {
  LineChart,
  Line,
  AreaChart,
  Area,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
  Legend,
} from 'recharts';

const axisProps = {
  stroke: 'var(--color-text-tertiary)',
  tick: { fill: 'var(--color-text-tertiary)', fontSize: 11 },
  axisLine: false,
  tickLine: false,
};

const gridProps = {
  strokeDasharray: '3 3',
  horizontal: true,
  vertical: false,
  stroke: 'var(--color-separator)',
};

const tooltipStyle = {
  contentStyle: {
    backgroundColor: 'var(--color-surface-raised)',
    border: '1px solid var(--color-border, hsl(var(--border)))',
    borderRadius: 6,
    color: 'hsl(var(--foreground))',
    fontSize: 12,
  },
  labelStyle: { color: 'var(--color-text-secondary)' },
};

function Heatmap({ data }: { data: HeatmapEntry[] }) {
  const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

  const { grid, maxReqs } = useMemo(() => {
    const index = new Map<string, HeatmapEntry>();
    let max = 1;
    for (const d of data) {
      index.set(`${d.day_of_week}-${d.hour}`, d);
      if (d.requests > max) max = d.requests;
    }
    const g = Array.from({ length: 7 }, (_, day) =>
      Array.from({ length: 24 }, (_, hour) =>
        index.get(`${day}-${hour}`) || { day_of_week: day, hour, requests: 0, cost: 0 }
      ),
    );
    return { grid: g, maxReqs: max };
  }, [data]);

  return (
    <div className="overflow-x-auto">
      <div style={{ minWidth: 600 }}>
        <div className="grid gap-1.5" style={{ gridTemplateColumns: 'auto repeat(24, 1fr)' }}>
          {/* header row with hours */}
          <div />
          {Array.from({ length: 24 }, (_, h) => (
            <div key={h} className="text-xs text-[var(--color-text-tertiary)] text-center">
              {h}
            </div>
          ))}
          {/* data rows */}
          {grid.map((row, dayIdx) => (
            <Fragment key={dayIdx}>
              <div className="text-xs text-[var(--color-text-tertiary)] pr-2">{days[dayIdx]}</div>
              {row.map((cell, hour) => {
                const intensity = cell.requests / maxReqs;
                return (
                  <div
                    key={`${dayIdx}-${hour}`}
                    className="aspect-square rounded-sm"
                    style={{
                      backgroundColor: `color-mix(in srgb, ${CHART_COLORS.primary} ${Math.round((intensity * 0.8 + (intensity > 0 ? 0.1 : 0)) * 100)}%, transparent)`,
                    }}
                    title={`${days[dayIdx]} ${hour}:00 — ${cell.requests} requests, ${formatCost(cell.cost)}`}
                  />
                );
              })}
            </Fragment>
          ))}
        </div>
      </div>
    </div>
  );
}

function buildProviderStackedData(points: ProviderTimePoint[]) {
  const providers = [...new Set(points.map((p) => p.provider))];
  const byTimestamp = new Map<string, Record<string, string | number>>();

  for (const pt of points) {
    if (!byTimestamp.has(pt.timestamp)) {
      byTimestamp.set(pt.timestamp, { timestamp: pt.timestamp });
    }
    byTimestamp.get(pt.timestamp)![pt.provider] = pt.cost;
  }

  const data = Array.from(byTimestamp.values());
  return { data, providers };
}

export default function Analytics() {
  const { range } = useTimeRange();
  const fetcher = useCallback(
    () => fetchAnalytics({ from: range.from, to: range.to }),
    [range.from, range.to],
  );
  const { data, loading, error, refresh } = usePolling(fetcher, 2000);

  const { data: providerData, providers } = useMemo(() => {
    if (!data) return { data: [], providers: [] };
    return buildProviderStackedData(data.by_provider_over_time);
  }, [data]);

  if (loading && !data) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2Icon className="size-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error && !data) {
    return (
      <div className="space-y-6">
        <DateRangePicker />
        <EmptyState
          icon={<span className="text-5xl">⚡</span>}
          title="Couldn't load analytics"
          description="The server might be warming up. Give it a moment."
          action={<Button variant="outline" size="sm" onClick={refresh}>Retry</Button>}
        />
      </div>
    );
  }

  if (!data || data.over_time.length === 0) {
    return (
      <div className="space-y-6">
        <DateRangePicker />
        <EmptyState icon={<span className="text-5xl">📊</span>} title="No analytics data yet" description="Start routing requests through the proxy to see charts light up." />
      </div>
    );
  }

  const { over_time, cumulative_cost, cache_hit_rate, avg_tokens_per_request, top_models, top_expensive, cost_distribution, heatmap } = data;

  // Derive summary metrics
  const totalCost = over_time.reduce((s, p) => s + p.cost, 0);
  const totalRequests = over_time.reduce((s, p) => s + p.requests, 0);
  const avgCostPerReq = totalRequests > 0 ? totalCost / totalRequests : 0;
  const totalTokens = over_time.reduce((s, p) => s + p.input_tokens + p.output_tokens, 0);
  const totalCacheRead = over_time.reduce((s, p) => s + p.cache_read_tokens, 0);
  const totalInput = over_time.reduce((s, p) => s + p.input_tokens, 0);
  const cacheHitPct = totalInput > 0 ? (totalCacheRead / totalInput) * 100 : 0;
  const avgTokensPerReq = totalRequests > 0 ? totalTokens / totalRequests : 0;

  return (
    <div className="space-y-6 animate-stagger">
      <DateRangePicker />

      <Tabs defaultValue="cost">
        <TabsList>
          <TabsTrigger value="cost">Cost</TabsTrigger>
          <TabsTrigger value="tokens">Tokens</TabsTrigger>
          <TabsTrigger value="performance">Performance</TabsTrigger>
        </TabsList>

        {/* Cost Tab */}
        <TabsContent value="cost" className="space-y-8 mt-6 animate-fade-in">
          {/* Summary strip */}
          <div className="flex flex-wrap gap-x-10 gap-y-3 pb-4 border-b border-[var(--color-separator)]">
            <div>
              <p className="text-[var(--text-micro)] uppercase tracking-wide text-[var(--color-text-tertiary)]">Total Spend</p>
              <p className="text-xl font-semibold text-foreground tabular-nums">{formatCost(totalCost)}</p>
            </div>
            <div>
              <p className="text-[var(--text-micro)] uppercase tracking-wide text-[var(--color-text-tertiary)]">Avg / Request</p>
              <p className="text-xl font-semibold text-foreground tabular-nums">{formatCost(avgCostPerReq)}</p>
            </div>
            <div>
              <p className="text-[var(--text-micro)] uppercase tracking-wide text-[var(--color-text-tertiary)]">P95 Cost</p>
              <p className="text-xl font-semibold text-foreground tabular-nums">{formatCost(cost_distribution.p95)}</p>
            </div>
            <div>
              <p className="text-[var(--text-micro)] uppercase tracking-wide text-[var(--color-text-tertiary)]">Max Cost</p>
              <p className="text-xl font-semibold text-foreground tabular-nums">{formatCost(cost_distribution.max)}</p>
            </div>
          </div>
          {/* Cost Over Time + Cumulative Cost side by side */}
          <div className="grid gap-8 md:grid-cols-2">
            <section>
              <SectionHeading>Cost Over Time</SectionHeading>
              <ResponsiveContainer width="100%" height={340}>
                <LineChart data={over_time}>
                  <CartesianGrid {...gridProps} />
                  <XAxis
                    dataKey="timestamp"
                    tickFormatter={(v: string) => formatDate(v)}
                    {...axisProps}
                  />
                  <YAxis tickFormatter={(v: number) => formatCost(v)} {...axisProps} />
                  <Tooltip
                    {...tooltipStyle}
                    labelFormatter={(v) => formatDate(String(v))}
                    formatter={(v) => [formatCost(Number(v)), 'Cost']}
                  />
                  <Line type="monotone" dataKey="cost" stroke={CHART_COLORS.cost} strokeWidth={2} dot={false} />
                </LineChart>
              </ResponsiveContainer>
            </section>

            <section>
              <SectionHeading>Cumulative Cost</SectionHeading>
              <ResponsiveContainer width="100%" height={340}>
                <AreaChart data={cumulative_cost}>
                  <defs>
                    <linearGradient id="cumulativeGrad" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={CHART_COLORS.cost} stopOpacity={0.3} />
                      <stop offset="100%" stopColor={CHART_COLORS.cost} stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid {...gridProps} />
                  <XAxis
                    dataKey="timestamp"
                    tickFormatter={(v: string) => formatDate(v)}
                    {...axisProps}
                  />
                  <YAxis tickFormatter={(v: number) => formatCost(v)} {...axisProps} />
                  <Tooltip
                    {...tooltipStyle}
                    labelFormatter={(v) => formatDate(String(v))}
                    formatter={(v) => [formatCost(Number(v)), 'Cumulative']}
                  />
                  <Area type="monotone" dataKey="cumulative" stroke={CHART_COLORS.cost} fill="url(#cumulativeGrad)" strokeWidth={2} />
                </AreaChart>
              </ResponsiveContainer>
            </section>
          </div>

          {/* Cost by Provider Over Time - full width */}
          <section>
            <SectionHeading>Cost by Provider Over Time</SectionHeading>
            <ResponsiveContainer width="100%" height={340}>
              <BarChart data={providerData}>
                <CartesianGrid {...gridProps} />
                <XAxis
                  dataKey="timestamp"
                  tickFormatter={(v: string) => formatDate(v)}
                  {...axisProps}
                />
                <YAxis tickFormatter={(v: number) => formatCost(v)} {...axisProps} />
                <Tooltip
                  {...tooltipStyle}
                  labelFormatter={(v) => formatDate(String(v))}
                  formatter={(v, name) => [formatCost(Number(v)), String(name)]}
                />
                <Legend />
                {providers.map((provider, i) => (
                  <Bar
                    key={provider}
                    dataKey={provider}
                    stackId="provider"
                    fill={getProviderColor(provider, i)}
                  />
                ))}
              </BarChart>
            </ResponsiveContainer>
          </section>

          {/* Cost Distribution */}
          <section>
            <SectionHeading>Cost Distribution</SectionHeading>
            <div className="space-y-3">
              {(
                [
                  { label: 'p50', value: cost_distribution.p50 },
                  { label: 'p90', value: cost_distribution.p90 },
                  { label: 'p95', value: cost_distribution.p95 },
                  { label: 'p99', value: cost_distribution.p99 },
                  { label: 'max', value: cost_distribution.max },
                ] as const
              ).map((item) => {
                const pct = cost_distribution.max > 0 ? (item.value / cost_distribution.max) * 100 : 0;
                return (
                  <div key={item.label} className="flex items-center gap-3">
                    <span className="w-8 text-xs text-[var(--color-text-tertiary)] text-right font-mono">{item.label}</span>
                    <div className="flex-1 h-5 bg-border rounded overflow-hidden">
                      <div
                        className="h-full bg-emerald-500 rounded"
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                    <span className="w-16 text-xs text-[var(--color-text-secondary)] text-right font-mono tabular-nums">
                      {formatCost(item.value)}
                    </span>
                  </div>
                );
              })}
            </div>
          </section>

          {/* Top 10 Expensive Requests */}
          <section>
            <SectionHeading>Top 10 Expensive Requests</SectionHeading>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Time</TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>Cost</TableHead>
                  <TableHead>Tokens</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {top_expensive.map((req) => (
                  <TableRow key={req.id}>
                    <TableCell>
                      <Link to={`/requests/${req.id}`} className="text-emerald-400 hover:underline">
                        {formatDate(req.timestamp)}
                      </Link>
                    </TableCell>
                    <TableCell className="text-foreground max-w-48 truncate">{req.model}</TableCell>
                    <TableCell className="text-foreground tabular-nums">{formatCost(req.cost)}</TableCell>
                    <TableCell className="text-foreground tabular-nums">{formatTokens(req.total_tokens)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </section>
        </TabsContent>

        {/* Tokens Tab */}
        <TabsContent value="tokens" className="space-y-8 mt-6 animate-fade-in">
          {/* Summary strip */}
          <div className="flex flex-wrap gap-x-10 gap-y-3 pb-4 border-b border-[var(--color-separator)]">
            <div>
              <p className="text-[var(--text-micro)] uppercase tracking-wide text-[var(--color-text-tertiary)]">Total Tokens</p>
              <p className="text-xl font-semibold text-foreground tabular-nums">{formatTokens(totalTokens)}</p>
            </div>
            <div>
              <p className="text-[var(--text-micro)] uppercase tracking-wide text-[var(--color-text-tertiary)]">Cache Hit Rate</p>
              <p className="text-xl font-semibold text-foreground tabular-nums">{cacheHitPct.toFixed(1)}%</p>
            </div>
            <div>
              <p className="text-[var(--text-micro)] uppercase tracking-wide text-[var(--color-text-tertiary)]">Avg / Request</p>
              <p className="text-xl font-semibold text-foreground tabular-nums">{formatTokens(Math.round(avgTokensPerReq))}</p>
            </div>
          </div>

          {/* Tokens Over Time (stacked) - full width */}
          <section>
            <SectionHeading>Tokens Over Time</SectionHeading>
            <ResponsiveContainer width="100%" height={340}>
              <AreaChart data={over_time}>
                <defs>
                  <linearGradient id="inputGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={CHART_COLORS.input} stopOpacity={0.4} />
                    <stop offset="100%" stopColor={CHART_COLORS.input} stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="outputGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={CHART_COLORS.output} stopOpacity={0.4} />
                    <stop offset="100%" stopColor={CHART_COLORS.output} stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="cacheReadGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={CHART_COLORS.cacheRead} stopOpacity={0.4} />
                    <stop offset="100%" stopColor={CHART_COLORS.cacheRead} stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="cacheWriteGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={CHART_COLORS.cacheWrite} stopOpacity={0.4} />
                    <stop offset="100%" stopColor={CHART_COLORS.cacheWrite} stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid {...gridProps} />
                <XAxis
                  dataKey="timestamp"
                  tickFormatter={(v: string) => formatDate(v)}
                  {...axisProps}
                />
                <YAxis tickFormatter={(v: number) => formatTokens(v)} {...axisProps} />
                <Tooltip
                  {...tooltipStyle}
                  labelFormatter={(v) => formatDate(String(v))}
                  formatter={(v, name) => [formatTokens(Number(v)), String(name)]}
                />
                <Legend />
                <Area type="monotone" dataKey="input_tokens" name="Input" stackId="1" stroke={CHART_COLORS.input} fill="url(#inputGrad)" strokeWidth={1.5} />
                <Area type="monotone" dataKey="output_tokens" name="Output" stackId="1" stroke={CHART_COLORS.output} fill="url(#outputGrad)" strokeWidth={1.5} />
                <Area type="monotone" dataKey="cache_read_tokens" name="Cache Read" stackId="1" stroke={CHART_COLORS.cacheRead} fill="url(#cacheReadGrad)" strokeWidth={1.5} />
                <Area type="monotone" dataKey="cache_write_tokens" name="Cache Write" stackId="1" stroke={CHART_COLORS.cacheWrite} fill="url(#cacheWriteGrad)" strokeWidth={1.5} />
              </AreaChart>
            </ResponsiveContainer>
          </section>

          {/* Avg Tokens per Request by Model - full width */}
          <section>
            <SectionHeading>Avg Tokens per Request by Model</SectionHeading>
            <ResponsiveContainer width="100%" height={340}>
              <BarChart data={avg_tokens_per_request} layout="vertical">
                <CartesianGrid {...gridProps} />
                <XAxis type="number" tickFormatter={(v: number) => formatTokens(v)} {...axisProps} />
                <YAxis type="category" dataKey="model" width={140} {...axisProps} />
                <Tooltip
                  {...tooltipStyle}
                  formatter={(v, name) => [formatTokens(Number(v)), String(name)]}
                />
                <Legend />
                <Bar dataKey="avg_input" name="Avg Input" fill={CHART_COLORS.input} radius={[0, 4, 4, 0]} />
                <Bar dataKey="avg_output" name="Avg Output" fill={CHART_COLORS.output} radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </section>

          {/* Cache Hit Rate - full width */}
          <section>
            <SectionHeading>Cache Hit Rate</SectionHeading>
            <ResponsiveContainer width="100%" height={340}>
              <LineChart data={cache_hit_rate}>
                <CartesianGrid {...gridProps} />
                <XAxis
                  dataKey="timestamp"
                  tickFormatter={(v: string) => formatDate(v)}
                  {...axisProps}
                />
                <YAxis domain={[0, 100]} tickFormatter={(v: number) => `${v}%`} {...axisProps} />
                <Tooltip
                  {...tooltipStyle}
                  labelFormatter={(v) => formatDate(String(v))}
                  formatter={(v) => [`${Number(v).toFixed(1)}%`, 'Cache Hit Rate']}
                />
                <Line type="monotone" dataKey="rate" stroke={CHART_COLORS.cacheHit} strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
          </section>
        </TabsContent>

        {/* Performance Tab */}
        <TabsContent value="performance" className="space-y-8 mt-6 animate-fade-in">
          {/* Summary strip */}
          <div className="flex flex-wrap gap-x-10 gap-y-3 pb-4 border-b border-[var(--color-separator)]">
            <div>
              <p className="text-[var(--text-micro)] uppercase tracking-wide text-[var(--color-text-tertiary)]">Total Requests</p>
              <p className="text-xl font-semibold text-foreground tabular-nums">{totalRequests.toLocaleString()}</p>
            </div>
            <div>
              <p className="text-[var(--text-micro)] uppercase tracking-wide text-[var(--color-text-tertiary)]">Models Used</p>
              <p className="text-xl font-semibold text-foreground tabular-nums">{top_models.length}</p>
            </div>
          </div>

          {/* Avg Response Time by Model - full width */}
          <section>
            <SectionHeading>Avg Response Time by Model</SectionHeading>
            <ResponsiveContainer width="100%" height={340}>
              <BarChart data={top_models} layout="vertical">
                <CartesianGrid {...gridProps} />
                <XAxis
                  type="number"
                  tickFormatter={(v: number) => formatDuration(v)}
                  {...axisProps}
                />
                <YAxis type="category" dataKey="model" width={140} {...axisProps} />
                <Tooltip
                  {...tooltipStyle}
                  formatter={(v) => [formatDuration(Number(v)), 'Avg Duration']}
                />
                <Bar dataKey="avg_duration_ms" name="Avg Duration" fill={CHART_COLORS.latency} radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </section>

          {/* Request Heatmap - full width */}
          <section>
            <SectionHeading>Request Heatmap</SectionHeading>
            <Heatmap data={heatmap} />
          </section>
        </TabsContent>
      </Tabs>
    </div>
  );
}
