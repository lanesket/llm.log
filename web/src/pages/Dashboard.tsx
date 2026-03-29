import { useState, useEffect, useMemo, useCallback } from 'react';
import { Loader2Icon } from 'lucide-react';
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';

import { usePolling } from '@/hooks/usePolling';
import { useTimeRange } from '@/hooks/useTimeRange';
import { useChameleon } from '@/hooks/useChameleon';
import { fetchDashboard, fetchStatus } from '@/lib/api';
import { formatCost, formatTokens, formatDelta, formatDate, formatAnimatedTokens, formatAnimatedCost, formatAnimatedNumber, formatAnimatedPercent } from '@/lib/utils';
import { useAnimatedValue } from '@/hooks/useAnimatedValue';
import { getProviderColor, CHART_COLORS } from '@/lib/constants';
import { DateRangePicker } from '@/components/DateRangePicker';
import { SectionHeading } from '@/components/SectionHeading';
import { ContributionHeatmap } from '@/components/ContributionHeatmap';
import { CopyableValue } from '@/components/CopyableValue';
import { EmptyState } from '@/components/EmptyState';
import { ProxyControl } from '@/components/ProxyControl';
import { Button } from '@/components/ui/button';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '@/components/ui/table';

type ChartTab = 'requests' | 'cost' | 'tokens';

// --- Metric strip item ---
interface MetricItemProps {
  label: string;
  rawValue: number;
  formatter: (n: number) => string;
  delta?: { text: string; positive: boolean } | null;
}

function MetricItem({ label, rawValue, formatter, delta }: MetricItemProps) {
  const animated = useAnimatedValue(rawValue);

  return (
    <div className="relative">
      <p className="text-[var(--text-micro)] uppercase tracking-wide text-[var(--color-text-tertiary)] mb-2">
        {label}
      </p>
      <p className="text-[var(--text-display)] font-semibold text-foreground tabular-nums leading-none">
        {formatter(animated)}
      </p>
      {delta && (
        <p className={`text-[var(--text-small)] mt-2 font-medium ${delta.positive ? 'text-emerald-400' : 'text-red-400'}`}>
          {delta.text} <span className="text-[var(--color-text-tertiary)] font-normal">vs prev</span>
        </p>
      )}
      {/* Accent underline — provider colored */}
      <div className="mt-3 h-0.5 w-10 rounded-full bg-[var(--accent-provider)] opacity-40" />
    </div>
  );
}

// --- Chart tooltip ---
interface ChartTooltipProps {
  active?: boolean;
  payload?: Array<{ value: number }>;
  label?: string;
  tab: ChartTab;
}

function ChartTooltip({ active, payload, label, tab }: ChartTooltipProps) {
  if (!active || !payload?.length) return null;
  const raw = payload[0].value;
  let formatted: string;
  if (tab === 'cost') formatted = formatCost(raw);
  else if (tab === 'tokens') formatted = formatTokens(raw);
  else formatted = String(raw);

  return (
    <div className="rounded-md border border-border bg-[var(--color-surface-raised)] px-3 py-2 text-sm text-foreground shadow">
      <p className="text-xs text-[var(--color-text-secondary)]">{label ? formatDate(label) : ''}</p>
      <p className="font-medium">{formatted}</p>
    </div>
  );
}

export default function Dashboard() {
  const { range } = useTimeRange();
  const fetcher = useCallback(
    () => fetchDashboard(range.from, range.to),
    [range.from, range.to],
  );
  const { data, loading, error, refresh } = usePolling(fetcher, 2000);

  const [chartTab, setChartTab] = useState<ChartTab>('requests');
  const { data: status } = usePolling(fetchStatus, 10000);

  // Feed chameleon data
  const { feedData } = useChameleon();
  useEffect(() => {
    if (data) {
      const dominantProvider = data.by_provider[0]?.name;
      feedData(data.totals.requests, status?.proxy_running ?? false, dominantProvider);
      // Set CSS accent color to match dominant provider
      if (dominantProvider) {
        const color = getProviderColor(dominantProvider.toLowerCase(), 0);
        document.documentElement.style.setProperty('--accent-provider', color);
      }
    }
  }, [data, status, feedData]);

  const topModels = useMemo(() => {
    if (!data) return [];
    return [...data.by_model]
      .sort((a, b) => b.total_cost - a.total_cost)
      .slice(0, 5);
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
          title="Couldn't load dashboard"
          description="The server might be warming up. Give it a moment."
          action={<Button variant="outline" size="sm" onClick={refresh}>Retry</Button>}
        />
      </div>
    );
  }

  if (!data || data.totals.requests === 0) {
    return (
      <div className="space-y-6">
        <DateRangePicker />
        <EmptyState
          icon={<span className="text-5xl">🦎</span>}
          title="No requests yet"
          description="Start your proxy and make some LLM calls. Your chameleon is getting bored."
          action={<ProxyControl />}
        />
      </div>
    );
  }

  const { totals, prev_totals, by_provider, chart } = data;

  const totalTokens = totals.input_tokens + totals.output_tokens;
  const prevTotalTokens = prev_totals
    ? prev_totals.input_tokens + prev_totals.output_tokens
    : 0;
  const errorRate =
    totals.requests > 0 ? (totals.errors / totals.requests) * 100 : 0;
  const prevErrorRate =
    prev_totals && prev_totals.requests > 0
      ? (prev_totals.errors / prev_totals.requests) * 100
      : 0;

  const requestsDelta = prev_totals
    ? formatDelta(totals.requests, prev_totals.requests)
    : null;
  const costDelta = prev_totals
    ? formatDelta(totals.total_cost, prev_totals.total_cost)
    : null;
  const tokensDelta = prev_totals
    ? formatDelta(totalTokens, prevTotalTokens)
    : null;
  const errorDelta = prev_totals
    ? formatDelta(errorRate, prevErrorRate)
    : null;

  // Provider breakdown percentages
  const providerTotal = by_provider.reduce((sum, p) => sum + p.total_cost, 0);

  return (
    <div className="space-y-6 animate-stagger">
      <DateRangePicker />

      {/* Metrics */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-8" data-chameleon-perch>
        <MetricItem
          label="Total Requests"
          rawValue={totals.requests}
          formatter={formatAnimatedNumber}
          delta={requestsDelta}
        />
        <MetricItem
          label="Total Cost"
          rawValue={totals.total_cost}
          formatter={formatAnimatedCost}
          delta={costDelta}
        />
        <MetricItem
          label="Total Tokens"
          rawValue={totalTokens}
          formatter={formatAnimatedTokens}
          delta={tokensDelta}
        />
        <MetricItem
          label="Error Rate"
          rawValue={errorRate}
          formatter={formatAnimatedPercent}
          delta={errorDelta}
        />
      </div>

      {/* Contribution heatmap */}
      {data.activity?.length > 0 && (
        <div className="border-t border-[var(--color-separator)] pt-8 mt-8">
          <SectionHeading as="h2">Activity</SectionHeading>
          <ContributionHeatmap activity={data.activity} />
        </div>
      )}

      {/* Chart */}
      <div className="border-t border-[var(--color-separator)] pt-8 mt-8" data-chameleon-perch>
        <div className="flex items-center justify-between mb-6">
          <SectionHeading as="h2">Overview</SectionHeading>
          <Tabs
            value={chartTab}
            onValueChange={(v) => setChartTab(v as ChartTab)}
          >
            <TabsList>
              <TabsTrigger value="requests">Requests</TabsTrigger>
              <TabsTrigger value="cost">Cost</TabsTrigger>
              <TabsTrigger value="tokens">Tokens</TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
        <div className="h-[280px] sm:h-[400px]">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={chart}>
            <defs>
              <linearGradient id="areaGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={CHART_COLORS.primary} stopOpacity={0.3} />
                <stop offset="100%" stopColor={CHART_COLORS.primary} stopOpacity={0} />
              </linearGradient>
            </defs>
            <XAxis
              dataKey="timestamp"
              tickFormatter={(v: string) => formatDate(v)}
              stroke="transparent"
              tick={{ fill: 'var(--color-text-secondary)', fontSize: 12 }}
              axisLine={false}
              tickLine={false}
            />
            <YAxis
              stroke="transparent"
              tick={{ fill: 'var(--color-text-secondary)', fontSize: 12 }}
              axisLine={false}
              tickLine={false}
              tickFormatter={(v: number) => {
                if (chartTab === 'cost') return formatCost(v);
                if (chartTab === 'tokens') return formatTokens(v);
                return String(v);
              }}
            />
            <Tooltip
              content={<ChartTooltip tab={chartTab} />}
              cursor={{ stroke: 'var(--color-separator)' }}
            />
            <Area
              type="monotone"
              dataKey={chartTab}
              stroke={CHART_COLORS.primary}
              fill="url(#areaGradient)"
              strokeWidth={2}
            />
          </AreaChart>
        </ResponsiveContainer>
        </div>
      </div>

      {/* Bottom row: provider bars + top models table */}
      <div className="grid gap-8 md:grid-cols-2">
        {/* Provider breakdown — horizontal bars */}
        <div>
          <SectionHeading as="h2">By Provider</SectionHeading>
          <div className="flex flex-col gap-3">
            {by_provider.map((p, i) => {
              const pct = providerTotal > 0 ? (p.total_cost / providerTotal) * 100 : 0;
              return (
                <div key={p.name} className="flex items-center gap-3">
                  <span className="w-24 text-[var(--text-small)] text-[var(--color-text-secondary)] truncate">{p.name}</span>
                  <div className="flex-1 h-2.5 rounded-full bg-[var(--color-surface-raised)] overflow-hidden">
                    <div
                      className="h-full rounded-full transition-all duration-500"
                      style={{ width: `${pct}%`, backgroundColor: getProviderColor(p.name, i) }}
                    />
                  </div>
                  <span className="w-16 text-right text-[var(--text-small)] tabular-nums text-foreground">{formatCost(p.total_cost)}</span>
                  <span className="w-10 text-right text-[var(--text-micro)] text-[var(--color-text-tertiary)]">{pct.toFixed(0)}%</span>
                </div>
              );
            })}
          </div>
        </div>

        {/* Top models — compact table */}
        <div>
          <SectionHeading as="h2">Top Models by Cost</SectionHeading>
          <Table>
            <TableHeader>
              <TableRow className="border-border hover:bg-transparent">
                <TableHead className="text-xs text-[var(--color-text-tertiary)]">Model</TableHead>
                <TableHead className="text-xs text-[var(--color-text-tertiary)]">Provider</TableHead>
                <TableHead className="text-xs text-[var(--color-text-tertiary)] text-right">Requests</TableHead>
                <TableHead className="text-xs text-[var(--color-text-tertiary)] text-right">Cost</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {topModels.map((m) => (
                <TableRow key={m.name} className="border-border">
                  <TableCell className="text-xs text-foreground max-w-40 truncate">
                    <CopyableValue value={m.name} className="text-xs text-foreground" />
                  </TableCell>
                  <TableCell className="text-xs text-[var(--color-text-secondary)] max-w-28 truncate">{m.provider ?? '—'}</TableCell>
                  <TableCell className="text-xs text-foreground tabular-nums text-right">{m.requests.toLocaleString()}</TableCell>
                  <TableCell className="text-xs text-foreground tabular-nums text-right">{formatCost(m.total_cost)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </div>
    </div>
  );
}
