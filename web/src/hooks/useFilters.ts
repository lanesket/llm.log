import { useState, useMemo } from 'react';
import type { RequestsParams } from '@/lib/types';

export function useFilters() {
  const [provider, setProvider] = useState('');
  const [model, setModel] = useState('');
  const [source, setSource] = useState('');
  const [statusCode, setStatusCode] = useState<number | undefined>();
  const [streaming, setStreaming] = useState<boolean | undefined>();
  const [minCost, setMinCost] = useState<number | undefined>();
  const [maxCost, setMaxCost] = useState<number | undefined>();
  const [minTokens, setMinTokens] = useState<number | undefined>();
  const [maxTokens, setMaxTokens] = useState<number | undefined>();
  const [search, setSearch] = useState('');
  const [sort, setSort] = useState('timestamp');
  const [dir, setDir] = useState<'asc' | 'desc'>('desc');

  const params = useMemo((): Partial<RequestsParams> => {
    const p: Partial<RequestsParams> = { sort, dir };
    if (provider) p.provider = provider;
    if (model) p.model = model;
    if (source) p.source = source;
    if (statusCode) p.status_code = statusCode;
    if (streaming !== undefined) p.streaming = streaming;
    if (minCost !== undefined) p.min_cost = minCost;
    if (maxCost !== undefined) p.max_cost = maxCost;
    if (minTokens !== undefined) p.min_tokens = minTokens;
    if (maxTokens !== undefined) p.max_tokens = maxTokens;
    if (search) p.search = search;
    return p;
  }, [provider, model, source, statusCode, streaming, minCost, maxCost, minTokens, maxTokens, search, sort, dir]);

  const clearAll = () => {
    setProvider(''); setModel(''); setSource('');
    setStatusCode(undefined); setStreaming(undefined);
    setMinCost(undefined); setMaxCost(undefined);
    setMinTokens(undefined); setMaxTokens(undefined);
    setSearch('');
  };

  const toggleSort = (column: string) => {
    if (sort === column) {
      setDir(d => d === 'asc' ? 'desc' : 'asc');
    } else {
      setSort(column);
      setDir('desc');
    }
  };

  return {
    provider, setProvider, model, setModel, source, setSource,
    statusCode, setStatusCode, streaming, setStreaming,
    minCost, setMinCost, maxCost, setMaxCost,
    minTokens, setMinTokens, maxTokens, setMaxTokens,
    search, setSearch, sort, dir, toggleSort,
    params, clearAll,
  };
}
