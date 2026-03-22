import { useTimeRange, type Preset } from '@/hooks/useTimeRange';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

const presets: { label: string; value: Preset }[] = [
  { label: '1h', value: '1h' },
  { label: 'Today', value: 'today' },
  { label: 'Yesterday', value: 'yesterday' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
  { label: 'Custom', value: 'custom' },
];

function toLocalDatetime(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function DateRangePicker() {
  const { range, setPreset, setCustom } = useTimeRange();

  return (
    <div className="flex flex-wrap items-center gap-2">
      <div className="flex gap-1">
        {presets.map((p) => (
          <Button
            key={p.value}
            variant={range.preset === p.value ? 'secondary' : 'ghost'}
            size="sm"
            onClick={() => {
              if (p.value !== 'custom') {
                setPreset(p.value);
              } else {
                setCustom(range.from, range.to);
              }
            }}
          >
            {p.label}
          </Button>
        ))}
      </div>
      {range.preset === 'custom' && (
        <div className="flex items-center gap-2">
          <Input
            type="datetime-local"
            aria-label="Start date"
            value={toLocalDatetime(range.from)}
            onChange={(e) => {
              const val = e.target.value;
              if (val) setCustom(new Date(val).toISOString(), range.to);
            }}
            className="h-7 w-auto text-xs"
          />
          <span className="text-[var(--color-text-secondary)] text-xs">to</span>
          <Input
            type="datetime-local"
            aria-label="End date"
            value={toLocalDatetime(range.to)}
            onChange={(e) => {
              const val = e.target.value;
              if (val) setCustom(range.from, new Date(val).toISOString());
            }}
            className="h-7 w-auto text-xs"
          />
        </div>
      )}
    </div>
  );
}
