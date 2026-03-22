import { createContext, useContext, useState, useMemo, useEffect, type ReactNode } from 'react';

export type Preset = '1h' | 'today' | 'yesterday' | '7d' | '30d' | 'custom';

export interface TimeRange {
  from: string;
  to: string;
  preset: Preset;
}

function computeRange(preset: Preset): { from: string; to: string } {
  const now = new Date();
  const fmt = (d: Date) => d.toISOString();

  switch (preset) {
    case '1h':
      return { from: fmt(new Date(now.getTime() - 60 * 60 * 1000)), to: fmt(now) };
    case 'today': {
      const start = new Date(now);
      start.setHours(0, 0, 0, 0);
      return { from: fmt(start), to: fmt(now) };
    }
    case 'yesterday': {
      const start = new Date(now);
      start.setDate(start.getDate() - 1);
      start.setHours(0, 0, 0, 0);
      const end = new Date(start);
      end.setHours(23, 59, 59, 999);
      return { from: fmt(start), to: fmt(end) };
    }
    case '7d':
      return { from: fmt(new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000)), to: fmt(now) };
    case '30d':
      return { from: fmt(new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)), to: fmt(now) };
    default:
      return { from: fmt(new Date(now.getTime() - 24 * 60 * 60 * 1000)), to: fmt(now) };
  }
}

interface TimeRangeContextType {
  range: TimeRange;
  setPreset: (p: Preset) => void;
  setCustom: (from: string, to: string) => void;
}

const TimeRangeContext = createContext<TimeRangeContextType | null>(null);

export function TimeRangeProvider({ children }: { children: ReactNode }) {
  const [preset, setPresetState] = useState<Preset>('today');
  const [customFrom, setCustomFrom] = useState('');
  const [customTo, setCustomTo] = useState('');
  // Tick counter to refresh non-custom ranges periodically
  const [tick, setTick] = useState(0);

  // Refresh `to` every 30s for non-custom presets so polling picks up new data
  useEffect(() => {
    if (preset === 'custom' || preset === 'yesterday') return;
    const timer = setInterval(() => setTick(t => t + 1), 30_000);
    return () => clearInterval(timer);
  }, [preset]);

  const range = useMemo((): TimeRange => {
    if (preset === 'custom' && customFrom && customTo) {
      return { from: customFrom, to: customTo, preset: 'custom' };
    }
    const { from, to } = computeRange(preset);
    return { from, to, preset };
  }, [preset, customFrom, customTo, tick]); // eslint-disable-line react-hooks/exhaustive-deps

  const setPreset = (p: Preset) => { setPresetState(p); setTick(t => t + 1); };
  const setCustom = (from: string, to: string) => {
    setCustomFrom(from);
    setCustomTo(to);
    setPresetState('custom');
  };

  return (
    <TimeRangeContext.Provider value={{ range, setPreset, setCustom }}>
      {children}
    </TimeRangeContext.Provider>
  );
}

export function useTimeRange() {
  const ctx = useContext(TimeRangeContext);
  if (!ctx) throw new Error('useTimeRange must be used within TimeRangeProvider');
  return ctx;
}
