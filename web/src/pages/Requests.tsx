import { useState, useEffect, useMemo, useCallback } from 'react';
import { ArrowUpIcon, ArrowDownIcon, ExternalLinkIcon, Loader2Icon, ChevronLeftIcon, ChevronRightIcon } from 'lucide-react';
import { CopyableValue } from '@/components/CopyableValue';
import { DateRangePicker } from '@/components/DateRangePicker';
import { FilterBar } from '@/components/FilterBar';
import { JsonViewer } from '@/components/JsonViewer';
import { EmptyState } from '@/components/EmptyState';
import { useTimeRange } from '@/hooks/useTimeRange';
import { useFilters } from '@/hooks/useFilters';
import { fetchRequests, fetchRequestDetail } from '@/lib/api';
import { formatCost, formatTokens, formatDuration, formatDate } from '@/lib/utils';
import {
  Table, TableHeader, TableBody, TableHead, TableRow, TableCell,
} from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import type { RequestItem, RequestDetailResponse } from '@/lib/types';

const COLUMNS = [
  { key: 'timestamp', label: 'Time', hiddenOnMobile: false },
  { key: 'provider', label: 'Provider', hiddenOnMobile: false },
  { key: 'model', label: 'Model', hiddenOnMobile: false },
  { key: 'source', label: 'Source', hiddenOnMobile: true },
  { key: 'input_tokens', label: 'Input', hiddenOnMobile: true },
  { key: 'output_tokens', label: 'Output', hiddenOnMobile: true },
  { key: 'total_cost', label: 'Cost', hiddenOnMobile: false },
  { key: 'duration_ms', label: 'Duration', hiddenOnMobile: true },
  { key: 'status_code', label: 'Status', hiddenOnMobile: false },
];

const NUMERIC_COLUMNS = new Set(['input_tokens', 'output_tokens', 'total_cost', 'duration_ms']);
const PAGE_SIZE_OPTIONS = [25, 50, 100];

function SortArrow({ column, sort, dir }: { column: string; sort: string; dir: string }) {
  if (sort !== column) return null;
  return dir === 'asc' ? (
    <ArrowUpIcon className="inline size-3.5 ml-1" />
  ) : (
    <ArrowDownIcon className="inline size-3.5 ml-1" />
  );
}

export function Requests() {
  const { range } = useTimeRange();
  const filters = useFilters();

  const [items, setItems] = useState<RequestItem[]>([]);
  const [total, setTotal] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Pagination state
  const [pageSize, setPageSize] = useState(50);
  const [page, setPage] = useState(0); // 0-indexed
  // Store cursors for each page so we can go back
  const [pageCursors, setPageCursors] = useState<string[]>(['']); // index 0 = first page (no cursor)
  const [nextCursor, setNextCursor] = useState('');

  // Detail panel
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [detail, setDetail] = useState<RequestDetailResponse | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  // Reset pagination when filters/sort/time range change
  const paramsKey = useMemo(
    () => JSON.stringify({ ...filters.params, from: range.from, to: range.to }),
    [filters.params, range.from, range.to],
  );

  // Reset to page 0 when params change
  useEffect(() => {
    setPage(0);
    setPageCursors(['']);
    setNextCursor('');
  }, [paramsKey, pageSize]);

  // Fetch current page
  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      setLoading(true);
      setError(null);

      const cursor = pageCursors[page] ?? '';

      try {
        const res = await fetchRequests({
          ...filters.params,
          from: range.from,
          to: range.to,
          limit: pageSize,
          cursor: cursor || undefined,
        });
        if (!cancelled) {
          setItems(res.items);
          setTotal(res.total);
          setNextCursor(res.next_cursor);
          // Store the cursor for the next page
          if (res.next_cursor) {
            setPageCursors(prev => {
              const next = [...prev];
              next[page + 1] = res.next_cursor;
              return next;
            });
          }
        }
      } catch {
        if (!cancelled) setError('Failed to load requests');
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    load();
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [paramsKey, page, pageSize]);

  const totalPages = total !== null ? Math.ceil(total / pageSize) : null;
  const hasNext = !!nextCursor;
  const hasPrev = page > 0;

  const goNext = useCallback(() => { if (hasNext) setPage(p => p + 1); }, [hasNext]);
  const goPrev = useCallback(() => { if (hasPrev) setPage(p => p - 1); }, [hasPrev]);

  // Fetch detail when a row is selected
  useEffect(() => {
    if (selectedId === null) { setDetail(null); return; }
    let cancelled = false;
    setDetailLoading(true);
    fetchRequestDetail(selectedId)
      .then((res) => { if (!cancelled) setDetail(res); })
      .catch(() => {})
      .finally(() => { if (!cancelled) setDetailLoading(false); });
    return () => { cancelled = true; };
  }, [selectedId]);

  const showFrom = page * pageSize + 1;
  const showTo = page * pageSize + items.length;

  return (
    <div className="flex flex-col gap-4 animate-stagger">
      {/* Toolbar */}
      <div className="flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-foreground">Requests</h1>
          <DateRangePicker />
        </div>
        <FilterBar filters={filters} />
      </div>

      {/* Table */}
      {error ? (
        <EmptyState
          icon={<span className="text-5xl">⚡</span>}
          title="Connection hiccup"
          description="Couldn't reach the server. It might be taking a nap."
          action={<Button variant="outline" size="sm" onClick={() => window.location.reload()}>Retry</Button>}
        />
      ) : loading ? (
        <div className="flex items-center justify-center py-16">
          <Loader2Icon className="size-6 animate-spin text-muted-foreground" />
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<span className="text-5xl">🔍</span>}
          title="No requests found"
          description="Try widening your filters or picking a different time range."
        />
      ) : (
        <>
          <div className="overflow-x-auto" data-chameleon-perch>
          <Table>
            <TableHeader>
              <TableRow className="border-border hover:bg-transparent">
                {COLUMNS.map((col) => (
                  <TableHead
                    key={col.key}
                    className={`cursor-pointer select-none text-xs text-[var(--color-text-secondary)] hover:text-foreground transition-colors ${NUMERIC_COLUMNS.has(col.key) ? 'text-right' : ''} ${col.hiddenOnMobile ? 'hidden sm:table-cell' : ''}`}
                    aria-sort={filters.sort === col.key ? (filters.dir === 'asc' ? 'ascending' : 'descending') : 'none'}
                    onClick={() => filters.toggleSort(col.key)}
                  >
                    {col.label}
                    <SortArrow column={col.key} sort={filters.sort} dir={filters.dir} />
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item) => (
                <TableRow
                  key={item.id}
                  tabIndex={0}
                  className="cursor-pointer border-border hover:bg-[var(--color-surface-hover)] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-muted-foreground even:bg-[var(--color-surface-raised)]"
                  onClick={() => setSelectedId(item.id)}
                  onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setSelectedId(item.id); } }}
                >
                  <TableCell className="text-xs text-foreground">{formatDate(item.timestamp)}</TableCell>
                  <TableCell className="text-xs text-foreground max-w-28 truncate">{item.provider}</TableCell>
                  <TableCell className="text-xs text-foreground max-w-48 truncate">{item.model}</TableCell>
                  <TableCell className="hidden sm:table-cell text-xs text-[var(--color-text-secondary)] max-w-32 truncate">{item.source}</TableCell>
                  <TableCell className="hidden sm:table-cell text-xs text-[var(--color-text-secondary)] tabular-nums text-right">{formatTokens(item.input_tokens)}</TableCell>
                  <TableCell className="hidden sm:table-cell text-xs text-[var(--color-text-secondary)] tabular-nums text-right">{formatTokens(item.output_tokens)}</TableCell>
                  <TableCell className="text-xs text-foreground tabular-nums text-right">{formatCost(item.total_cost)}</TableCell>
                  <TableCell className="hidden sm:table-cell text-xs text-[var(--color-text-secondary)] tabular-nums text-right">{formatDuration(item.duration_ms)}</TableCell>
                  <TableCell>
                    <span className="flex items-center gap-1.5">
                      <span className={`inline-block w-1.5 h-1.5 rounded-full ${
                        item.status_code >= 400 ? 'bg-red-400' : item.status_code >= 300 ? 'bg-amber-400' : 'bg-emerald-400'
                      }`} />
                      <span className="tabular-nums text-xs">{item.status_code}</span>
                    </span>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          </div>

          {/* Pagination controls */}
          <div className="flex items-center justify-between pt-2">
            <div className="flex items-center gap-2 text-xs text-[var(--color-text-secondary)]">
              <span className="hidden sm:inline">Rows per page</span>
              <Select
                value={String(pageSize)}
                onValueChange={(val) => setPageSize(Number(val))}
              >
                <SelectTrigger className="h-7 w-[70px] text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PAGE_SIZE_OPTIONS.map(n => (
                    <SelectItem key={n} value={String(n)}>{n}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-center gap-3">
              <span className="text-xs text-[var(--color-text-secondary)] tabular-nums">
                {total !== null
                  ? `${showFrom}–${showTo} of ${total}`
                  : `${showFrom}–${showTo}`}
                {!hasNext && items.length > 0 && (
                  <span className="text-xs text-[var(--color-text-tertiary)] ml-2">· That's all!</span>
                )}
              </span>
              <div className="flex gap-1">
                <Button
                  variant="ghost"
                  size="icon-sm"
                  disabled={!hasPrev}
                  onClick={goPrev}
                  aria-label="Previous page"
                >
                  <ChevronLeftIcon className="size-4" />
                </Button>
                {totalPages !== null && (
                  <span className="flex items-center px-1 text-xs text-[var(--color-text-tertiary)] tabular-nums">
                    {page + 1} / {totalPages}
                  </span>
                )}
                <Button
                  variant="ghost"
                  size="icon-sm"
                  disabled={!hasNext}
                  onClick={goNext}
                  aria-label="Next page"
                >
                  <ChevronRightIcon className="size-4" />
                </Button>
              </div>
            </div>
          </div>
        </>
      )}

      {/* Request Detail Dialog — centered, no heavy blur */}
      <Dialog open={selectedId !== null} onOpenChange={(open) => { if (!open) setSelectedId(null); }}>
        <DialogContent className="sm:max-w-2xl max-h-[85vh] overflow-y-auto p-6">
          {detailLoading || !detail ? (
            <div className="flex items-center justify-center py-12">
              <Loader2Icon className="size-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <div className="flex flex-col gap-5 animate-stagger">
              {/* Header */}
              <DialogHeader>
                <div className="flex items-center gap-2 pr-8">
                  <DialogTitle className="text-lg">
                    <CopyableValue value={detail.model} className="text-foreground text-lg font-semibold" />
                  </DialogTitle>
                  <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium tabular-nums ${
                    detail.status_code >= 400 ? 'bg-red-400/10 text-red-400' : 'bg-emerald-400/10 text-emerald-400'
                  }`}>
                    {detail.status_code}
                  </span>
                </div>
                <div className="flex items-center gap-2 text-xs text-[var(--color-text-secondary)]">
                  <CopyableValue value={detail.provider} className="text-xs text-[var(--color-text-secondary)]" />
                  <span className="text-[var(--color-text-tertiary)]">·</span>
                  <span>{formatDate(detail.timestamp)}</span>
                  <span className="text-[var(--color-text-tertiary)]">·</span>
                  <span>{detail.streaming ? 'Streaming' : 'Non-streaming'}</span>
                </div>
              </DialogHeader>

              {/* Metrics strip */}
              <div className="flex flex-wrap gap-x-5 gap-y-2 py-3 border-y border-[var(--color-separator)]">
                {[
                  { label: 'Input', value: formatTokens(detail.input_tokens), raw: String(detail.input_tokens) },
                  { label: 'Output', value: formatTokens(detail.output_tokens), raw: String(detail.output_tokens) },
                  { label: 'Cost', value: formatCost(detail.total_cost), raw: detail.total_cost !== null ? String(detail.total_cost) : 'N/A' },
                  { label: 'Duration', value: formatDuration(detail.duration_ms), raw: `${detail.duration_ms}ms` },
                ].map(m => (
                  <div key={m.label} className="flex items-baseline gap-1.5">
                    <span className="text-[var(--text-micro)] uppercase tracking-wide text-[var(--color-text-tertiary)]">{m.label}</span>
                    <CopyableValue value={m.raw} display={m.value} className="text-sm font-medium text-foreground tabular-nums" />
                  </div>
                ))}
              </div>

              {/* Metadata */}
              <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-xs">
                <span className="text-[var(--color-text-tertiary)]">Endpoint</span>
                <CopyableValue value={detail.endpoint} className="text-xs text-foreground truncate" mono />
                <span className="text-[var(--color-text-tertiary)]">Source</span>
                <CopyableValue value={detail.source || '—'} className="text-xs text-foreground" />
                <span className="text-[var(--color-text-tertiary)]">ID</span>
                <CopyableValue value={String(detail.id)} className="text-xs text-foreground" mono />
              </div>

              {/* Open full view link */}
              <a
                href={`/requests/${detail.id}`}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 text-xs text-[var(--color-text-secondary)] hover:text-foreground transition-colors"
              >
                <ExternalLinkIcon className="size-3" />
                Open full view
              </a>

              {/* Body tabs */}
              <Tabs defaultValue="request">
                <TabsList>
                  <TabsTrigger value="request">Request</TabsTrigger>
                  <TabsTrigger value="response">Response</TabsTrigger>
                </TabsList>
                <TabsContent value="request" className="mt-3 max-h-[40vh] overflow-auto">
                  <JsonViewer data={detail.request_body || '{}'} />
                </TabsContent>
                <TabsContent value="response" className="mt-3 max-h-[40vh] overflow-auto">
                  <JsonViewer data={detail.response_body || '{}'} />
                </TabsContent>
              </Tabs>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
