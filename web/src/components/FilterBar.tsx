import { useCallback, useState, useEffect } from 'react';
import { SearchIcon, XIcon, AlertTriangleIcon } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useTimeRange } from '@/hooks/useTimeRange';
import { usePolling } from '@/hooks/usePolling';
import { fetchFilters } from '@/lib/api';
import type { useFilters } from '@/hooks/useFilters';

type Filters = ReturnType<typeof useFilters>;

interface FilterBarProps {
  filters: Filters;
}

export function FilterBar({ filters }: FilterBarProps) {
  const { range } = useTimeRange();

  // Debounce search input to avoid re-fetching on every keystroke
  const [searchInput, setSearchInput] = useState(filters.search);

  // Sync local state when filters.search is cleared externally (e.g. "Clear all")
  useEffect(() => {
    setSearchInput(filters.search);
  }, [filters.search]);

  useEffect(() => {
    const timer = setTimeout(() => filters.setSearch(searchInput), 300);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchInput]);

  const filtersFetcher = useCallback(
    () => fetchFilters(range.from, range.to),
    [range.from, range.to],
  );

  const { data: filterOptions } = usePolling(filtersFetcher, 10000);

  const activeFilters: { label: string; onRemove: () => void }[] = [];
  if (filters.provider) {
    activeFilters.push({ label: `Provider: ${filters.provider}`, onRemove: () => filters.setProvider('') });
  }
  if (filters.model) {
    activeFilters.push({ label: `Model: ${filters.model}`, onRemove: () => filters.setModel('') });
  }
  if (filters.source) {
    activeFilters.push({ label: `Source: ${filters.source}`, onRemove: () => filters.setSource('') });
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        {/* Search input */}
        <div className="relative">
          <SearchIcon className="absolute left-2.5 top-1/2 -translate-y-1/2 size-3.5 text-[var(--color-text-tertiary)]" />
          <Input
            placeholder="Search request/response bodies..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="h-8 w-full sm:w-64 pl-8 text-xs"
          />
        </div>

        {searchInput && (
          <Badge variant="outline" className="gap-1 text-amber-400 border-amber-400/30">
            <AlertTriangleIcon className="size-3" />
            Search may be slow for large datasets
          </Badge>
        )}

        {/* Provider select */}
        <Select
          value={filters.provider}
          onValueChange={(val) => filters.setProvider(val ?? '')}
        >
          <SelectTrigger size="sm" className="text-xs">
            <SelectValue placeholder="Provider" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">All Providers</SelectItem>
            {filterOptions?.providers.map((p) => (
              <SelectItem key={p} value={p}>
                {p}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {/* Model select */}
        <Select
          value={filters.model}
          onValueChange={(val) => filters.setModel(val ?? '')}
        >
          <SelectTrigger size="sm" className="text-xs">
            <SelectValue placeholder="Model" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">All Models</SelectItem>
            {filterOptions?.models.map((m) => (
              <SelectItem key={m} value={m}>
                {m}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {/* Source select */}
        <Select
          value={filters.source}
          onValueChange={(val) => filters.setSource(val ?? '')}
        >
          <SelectTrigger size="sm" className="text-xs">
            <SelectValue placeholder="Source" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">All Sources</SelectItem>
            {filterOptions?.sources.map((s) => (
              <SelectItem key={s} value={s}>
                {s}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Active filters as removable badges */}
      {activeFilters.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5">
          {activeFilters.map((f) => (
            <Badge key={f.label} variant="secondary" className="gap-1 text-xs animate-badge-in">
              {f.label}
              <button onClick={f.onRemove} className="ml-0.5 hover:text-foreground" aria-label={`Remove ${f.label}`}>
                <XIcon className="size-3" />
              </button>
            </Badge>
          ))}
          <Button variant="ghost" size="sm" className="h-5 text-xs text-[var(--color-text-tertiary)]" onClick={filters.clearAll}>
            Clear all
          </Button>
        </div>
      )}
    </div>
  );
}
