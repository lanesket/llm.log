import { useState, useCallback, useRef, useEffect } from 'react';
import { CheckIcon, CopyIcon } from 'lucide-react';

interface CopyableValueProps {
  value: string;
  display?: string;
  className?: string;
  mono?: boolean;
}

export function CopyableValue({ value, display, className = '', mono = false }: CopyableValueProps) {
  const [copied, setCopied] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  useEffect(() => () => { if (timerRef.current) clearTimeout(timerRef.current); }, []);

  const handleCopy = useCallback(async () => {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => setCopied(false), 1500);
  }, [value]);

  return (
    <button
      onClick={handleCopy}
      className={`group inline-flex items-center gap-1 rounded px-1 -mx-1 transition-colors hover:bg-[var(--color-surface-raised)] active:bg-[var(--color-surface-hover)] ${className}`}
      style={{ cursor: 'pointer' }}
      title={copied ? 'Copied!' : `Copy: ${value}`}
    >
      <span className={mono ? 'font-mono' : ''}>
        {copied ? (
          <span className="text-emerald-400">{display || value}</span>
        ) : (
          display || value
        )}
      </span>
      {copied ? (
        <CheckIcon className="size-3 text-emerald-400 shrink-0" />
      ) : (
        <CopyIcon className="size-3 opacity-0 group-hover:opacity-40 transition-opacity shrink-0" />
      )}
    </button>
  );
}
