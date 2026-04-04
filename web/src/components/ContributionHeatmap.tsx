import { useMemo, useState, useRef, useCallback, Fragment } from 'react';
import type { DailyActivity } from '@/lib/types';
import { formatCost, formatDateKey } from '@/lib/utils';
import { getProviderColor, CHART_COLORS } from '@/lib/constants';

const INTENSITY_COLORS = [
  'var(--color-surface-raised)',
  `color-mix(in srgb, ${CHART_COLORS.primary} 20%, transparent)`,
  `color-mix(in srgb, ${CHART_COLORS.primary} 45%, transparent)`,
  `color-mix(in srgb, ${CHART_COLORS.primary} 70%, transparent)`,
  CHART_COLORS.primary,
];

const CELL_SIZE = 15;
const GAP = 3;
const LABEL_COL = 40;
const CELL_RADIUS = '3px';
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

function formatHoverDate(dateStr: string): string {
  const d = new Date(dateStr + 'T00:00:00');
  return d.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' });
}

interface CellData {
  dateStr: string;
  requests: number;
  cost: number;
  activity?: DailyActivity;
}

interface TooltipState {
  cell: CellData;
  x: number;
  y: number;
  flipBelow: boolean;
}

interface Props {
  activity: DailyActivity[];
}

export function ContributionHeatmap({ activity }: Props) {
  const [tooltip, setTooltip] = useState<TooltipState | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const { grid, weeks, monthLabelMap, maxCost } = useMemo(() => {
    if (!activity.length) return { grid: [] as CellData[][], weeks: 0, monthLabelMap: new Map<number, string>(), maxCost: 0 };

    const lookup = new Map<string, DailyActivity>();
    let max = 0;
    for (const d of activity) {
      lookup.set(d.date, d);
      if (d.cost > max) max = d.cost;
    }

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
        const key = formatDateKey(d);
        const entry = lookup.get(key);
        g[row].push({
          dateStr: key,
          requests: entry?.requests ?? 0,
          cost: entry?.cost ?? 0,
          activity: entry,
        });
      }
    }

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

  const handleCellHover = useCallback((cell: CellData, e: React.MouseEvent) => {
    const el = e.currentTarget as HTMLElement;
    const container = containerRef.current;
    if (!container) return;
    const cellRect = el.getBoundingClientRect();
    const containerRect = container.getBoundingClientRect();
    const x = cellRect.left + cellRect.width / 2 - containerRect.left + container.scrollLeft;
    const y = cellRect.top - containerRect.top;
    const flipBelow = y < 60;
    setTooltip({ cell, x, y: flipBelow ? y + CELL_SIZE + 6 : y - 6, flipBelow });
  }, []);

  const handleCellLeave = useCallback(() => setTooltip(null), []);

  if (!activity.length || weeks === 0) return null;

  const cols = `${LABEL_COL}px repeat(${weeks}, ${CELL_SIZE}px)`;
  const hoveredCell = tooltip?.cell;
  const tooltipModels = hoveredCell?.activity?.models;

  return (
    <div className="relative" ref={containerRef}>
      <div className="overflow-x-auto">
        <div style={{ minWidth: weeks * (CELL_SIZE + GAP) + LABEL_COL }}>
          {/* Month labels */}
          <div className="grid mb-2" style={{ gridTemplateColumns: cols, gap: `${GAP}px` }}>
            <div />
            {Array.from({ length: weeks }, (_, w) => (
              <div key={w} className="text-[var(--text-micro)] text-[var(--color-text-tertiary)] leading-none overflow-visible whitespace-nowrap">
                {monthLabelMap.get(w) ?? ''}
              </div>
            ))}
          </div>

          {/* Grid */}
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
                <div className="flex items-center pr-2 text-[var(--text-micro)] text-[var(--color-text-tertiary)] leading-none select-none">
                  {DAY_LABELS[rowIdx]}
                </div>
                {row.map((cell, colIdx) => (
                  <div
                    key={`${rowIdx}-${colIdx}`}
                    className="transition-[outline] duration-150 cursor-default"
                    style={{
                      borderRadius: CELL_RADIUS,
                      backgroundColor: INTENSITY_COLORS[intensityLevel(cell.cost, maxCost)],
                      outline: hoveredCell?.dateStr === cell.dateStr ? '1.5px solid var(--ring)' : '1.5px solid transparent',
                      outlineOffset: '-1px',
                    }}
                    onMouseEnter={(e) => handleCellHover(cell, e)}
                    onMouseLeave={handleCellLeave}
                  />
                ))}
              </Fragment>
            ))}
          </div>
        </div>
      </div>

      {/* Legend */}
      <div className="flex items-center gap-2 pt-3 text-[var(--text-micro)] text-[var(--color-text-tertiary)]">
        <span>Less</span>
        <div className="flex items-center gap-1">
          {INTENSITY_COLORS.map((color, i) => (
            <div key={i} style={{ backgroundColor: color, width: CELL_SIZE, height: CELL_SIZE, borderRadius: CELL_RADIUS }} />
          ))}
        </div>
        <span>More</span>
      </div>

      {/* Tooltip — flips below when near top */}
      {tooltip && (
        <div
          className="absolute z-10 pointer-events-none rounded-md border border-border bg-[var(--color-surface-raised)] px-3 py-1.5 shadow-lg"
          style={{
            left: `clamp(80px, ${tooltip.x}px, calc(100% - 80px))`,
            top: tooltip.y,
            transform: tooltip.flipBelow ? 'translate(-50%, 0)' : 'translate(-50%, -100%)',
          }}
        >
          <p className="text-[var(--text-micro)] font-medium text-foreground whitespace-nowrap">
            {formatHoverDate(tooltip.cell.dateStr)}
          </p>
          {tooltip.cell.requests > 0 ? (
            <>
              <p className="text-[var(--text-micro)] text-[var(--color-text-secondary)] tabular-nums whitespace-nowrap">
                {tooltip.cell.requests} reqs · {formatCost(tooltip.cell.cost)}
              </p>
              {tooltipModels && tooltipModels.length > 0 && (
                <div className="mt-1 pt-1 border-t border-border/50 flex flex-col gap-0.5">
                  {tooltipModels.slice(0, 5).map((m, i) => (
                    <div key={i} className="flex items-center gap-1.5 text-[var(--text-micro)] text-[var(--color-text-secondary)] whitespace-nowrap">
                      <span className="inline-block w-1.5 h-1.5 rounded-full shrink-0" style={{ backgroundColor: getProviderColor(m.provider) }} />
                      <span className="max-w-[120px] truncate">{m.model.split('/').pop()}</span>
                      <span className="ml-auto tabular-nums">{m.requests} reqs</span>
                      <span className="tabular-nums">{formatCost(m.cost)}</span>
                    </div>
                  ))}
                  {(tooltip.cell.activity?.model_count ?? 0) > 5 && (
                    <p className="text-[var(--text-micro)] text-[var(--color-text-tertiary)] whitespace-nowrap">
                      +{(tooltip.cell.activity?.model_count ?? 0) - 5} more
                    </p>
                  )}
                </div>
              )}
            </>
          ) : (
            <p className="text-[var(--text-micro)] text-[var(--color-text-tertiary)] whitespace-nowrap">
              No activity
            </p>
          )}
        </div>
      )}
    </div>
  );
}
