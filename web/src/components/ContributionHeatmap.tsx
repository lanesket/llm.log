import { useMemo, useState, Fragment } from 'react';
import type { DailyActivity } from '@/lib/types';
import { formatCost } from '@/lib/utils';
import { CHART_COLORS } from '@/lib/constants';

// Emerald intensity scale — matches CHART_COLORS.primary used across the UI.
const INTENSITY_COLORS = [
  'var(--color-surface-raised)',
  `color-mix(in srgb, ${CHART_COLORS.primary} 20%, transparent)`,
  `color-mix(in srgb, ${CHART_COLORS.primary} 45%, transparent)`,
  `color-mix(in srgb, ${CHART_COLORS.primary} 70%, transparent)`,
  CHART_COLORS.primary,
];

const CELL_SIZE = 14;
const GAP = 3;
const LABEL_COL = 32;
const DAY_LABELS = ['Mon', '', 'Wed', '', 'Fri', '', ''];

function intensityLevel(cost: number, maxCost: number): number {
  if (cost === 0 || maxCost === 0) return 0;
  const ratio = cost / maxCost;
  if (ratio >= 0.75) return 4;
  if (ratio >= 0.50) return 3;
  if (ratio >= 0.25) return 2;
  return 1;
}

function mondayWeekStart(date: Date): Date {
  const d = new Date(date);
  d.setDate(d.getDate() - ((d.getDay() + 6) % 7));
  return d;
}

interface CellData {
  dateStr: string;
  requests: number;
  cost: number;
}

interface Props {
  activity: DailyActivity[];
}

export function ContributionHeatmap({ activity }: Props) {
  const [hoveredCell, setHoveredCell] = useState<CellData | null>(null);

  const { grid, weeks, monthLabelMap, maxCost } = useMemo(() => {
    if (!activity.length) return { grid: [] as CellData[][], weeks: 0, monthLabelMap: new Map<number, string>(), maxCost: 0 };

    const lookup = new Map<string, DailyActivity>();
    let max = 0;
    for (const d of activity) {
      lookup.set(d.date, d);
      if (d.cost > max) max = d.cost;
    }

    // Activity arrives sorted by date from the API.
    const earliest = new Date(activity[0].date + 'T00:00:00');
    const latest = new Date(activity[activity.length - 1].date + 'T00:00:00');

    const start = mondayWeekStart(earliest);
    const endDate = new Date(latest);
    const endDay = endDate.getDay();
    if (endDay !== 0) endDate.setDate(endDate.getDate() + (7 - endDay));

    const totalDays = Math.round((endDate.getTime() - start.getTime()) / (1000 * 60 * 60 * 24)) + 1;
    const totalWeeks = Math.ceil(totalDays / 7);

    const g: CellData[][] = Array.from({ length: 7 }, () => []);
    const months: { week: number; month: number; label: string }[] = [];
    let prevMonth = -1;
    const monthWeekCounts = new Map<number, number>();

    for (let w = 0; w < totalWeeks; w++) {
      const weekDate = new Date(start);
      weekDate.setDate(weekDate.getDate() + w * 7);
      const m = weekDate.getMonth();
      monthWeekCounts.set(m, (monthWeekCounts.get(m) ?? 0) + 1);
      if (m !== prevMonth) {
        months.push({ week: w, month: m, label: weekDate.toLocaleString('en', { month: 'short' }) });
        prevMonth = m;
      }

      for (let row = 0; row < 7; row++) {
        const d = new Date(start);
        d.setDate(d.getDate() + w * 7 + row);
        const key = d.toISOString().slice(0, 10);
        const entry = lookup.get(key);
        g[row].push({
          dateStr: key,
          requests: entry?.requests ?? 0,
          cost: entry?.cost ?? 0,
        });
      }
    }

    // Skip single-week padding months and prevent label overlap.
    const minLabelGap = 3;
    const result = new Map<number, string>();
    let lastEnd = -minLabelGap;
    for (const ml of months) {
      if ((monthWeekCounts.get(ml.month) ?? 0) < 2 && months.length > 1) continue;
      if (ml.week >= lastEnd + 1) {
        result.set(ml.week, ml.label);
        lastEnd = ml.week + minLabelGap - 1;
      }
    }

    return { grid: g, weeks: totalWeeks, monthLabelMap: result, maxCost: max };
  }, [activity]);

  if (!activity.length || weeks === 0) return null;

  const cols = `${LABEL_COL}px repeat(${weeks}, ${CELL_SIZE}px)`;

  return (
    <div className="overflow-x-auto">
      <div style={{ minWidth: weeks * (CELL_SIZE + GAP) + LABEL_COL }}>
        <div className="grid mb-1.5" style={{ gridTemplateColumns: cols, gap: `${GAP}px` }}>
          <div />
          {Array.from({ length: weeks }, (_, w) => (
            <div key={w} className="text-[var(--text-micro)] text-[var(--color-text-tertiary)] leading-none overflow-visible whitespace-nowrap">
              {monthLabelMap.get(w) ?? ''}
            </div>
          ))}
        </div>

        <div
          className="grid"
          style={{
            gridTemplateColumns: cols,
            gridTemplateRows: `repeat(7, ${CELL_SIZE}px)`,
            gap: `${GAP}px`,
          }}
        >
          {grid.map((row, rowIdx) => (
            <Fragment key={rowIdx}>
              <div className="flex items-center text-[var(--text-micro)] text-[var(--color-text-tertiary)] leading-none">
                {DAY_LABELS[rowIdx]}
              </div>
              {row.map((cell, colIdx) => (
                <div
                  key={`${rowIdx}-${colIdx}`}
                  className="rounded-sm transition-[outline] duration-150 cursor-default"
                  style={{
                    backgroundColor: INTENSITY_COLORS[intensityLevel(cell.cost, maxCost)],
                    outline: hoveredCell?.dateStr === cell.dateStr ? '1.5px solid var(--ring)' : '1.5px solid transparent',
                    outlineOffset: '-1px',
                  }}
                  onMouseEnter={() => setHoveredCell(cell)}
                  onMouseLeave={() => setHoveredCell(null)}
                />
              ))}
            </Fragment>
          ))}
        </div>
      </div>

      <div className="flex items-center justify-between pt-2">
        <div className="flex items-center gap-2 text-[var(--text-micro)] text-[var(--color-text-tertiary)]">
          <span>Less</span>
          <div className="flex items-center gap-1">
            {INTENSITY_COLORS.map((color, i) => (
              <div key={i} className="rounded-sm" style={{ backgroundColor: color, width: CELL_SIZE, height: CELL_SIZE }} />
            ))}
          </div>
          <span>More</span>
        </div>
        <div className="h-5 flex items-center text-[var(--text-micro)]">
          {hoveredCell && hoveredCell.requests > 0 && (
            <span className="text-[var(--color-text-secondary)] tabular-nums">
              <span className="font-medium text-foreground">{hoveredCell.dateStr}</span>
              {' \u2014 '}{hoveredCell.requests} requests, {formatCost(hoveredCell.cost)}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
