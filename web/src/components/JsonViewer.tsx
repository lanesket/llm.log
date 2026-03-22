import { useState, useMemo } from 'react';
import { Button } from '@/components/ui/button';
import { CopyIcon, CheckIcon } from 'lucide-react';

interface JsonViewerProps {
  data: string;
}

const SIZE_LIMIT = 100 * 1024; // 100KB

type JsonToken =
  | { type: 'key'; value: string }
  | { type: 'string'; value: string }
  | { type: 'number'; value: string }
  | { type: 'boolean'; value: string }
  | { type: 'null'; value: string }
  | { type: 'punctuation'; value: string };

function tokenize(obj: unknown, indent: number = 0): JsonToken[] {
  const tokens: JsonToken[] = [];
  const pad = '  '.repeat(indent);
  const padInner = '  '.repeat(indent + 1);

  if (obj === null) {
    tokens.push({ type: 'null', value: 'null' });
  } else if (typeof obj === 'boolean') {
    tokens.push({ type: 'boolean', value: String(obj) });
  } else if (typeof obj === 'number') {
    tokens.push({ type: 'number', value: String(obj) });
  } else if (typeof obj === 'string') {
    tokens.push({ type: 'string', value: JSON.stringify(obj) });
  } else if (Array.isArray(obj)) {
    if (obj.length === 0) {
      tokens.push({ type: 'punctuation', value: '[]' });
    } else {
      tokens.push({ type: 'punctuation', value: '[\n' });
      obj.forEach((item, i) => {
        tokens.push({ type: 'punctuation', value: padInner });
        tokens.push(...tokenize(item, indent + 1));
        if (i < obj.length - 1) {
          tokens.push({ type: 'punctuation', value: ',\n' });
        } else {
          tokens.push({ type: 'punctuation', value: '\n' });
        }
      });
      tokens.push({ type: 'punctuation', value: pad + ']' });
    }
  } else if (typeof obj === 'object') {
    const entries = Object.entries(obj as Record<string, unknown>);
    if (entries.length === 0) {
      tokens.push({ type: 'punctuation', value: '{}' });
    } else {
      tokens.push({ type: 'punctuation', value: '{\n' });
      entries.forEach(([key, val], i) => {
        tokens.push({ type: 'punctuation', value: padInner });
        tokens.push({ type: 'key', value: JSON.stringify(key) });
        tokens.push({ type: 'punctuation', value: ': ' });
        tokens.push(...tokenize(val, indent + 1));
        if (i < entries.length - 1) {
          tokens.push({ type: 'punctuation', value: ',\n' });
        } else {
          tokens.push({ type: 'punctuation', value: '\n' });
        }
      });
      tokens.push({ type: 'punctuation', value: pad + '}' });
    }
  }

  return tokens;
}

const colorMap: Record<string, string> = {
  key: 'text-[var(--color-text-secondary)]',
  string: 'text-emerald-400',
  number: 'text-blue-400',
  boolean: 'text-amber-400',
  null: 'text-amber-400',
  punctuation: 'text-[var(--color-text-tertiary)]',
};

export function JsonViewer({ data }: JsonViewerProps) {
  const [copied, setCopied] = useState(false);
  const [showFull, setShowFull] = useState(false);

  const safeData = data || '';
  const isLarge = safeData.length > SIZE_LIMIT;
  const displayData = isLarge && !showFull ? safeData.slice(0, SIZE_LIMIT) : safeData;

  const rendered = useMemo(() => {
    if (!displayData || displayData === 'null' || displayData === 'undefined') {
      return <span className="text-[var(--color-text-tertiary)] italic">No data</span>;
    }
    try {
      const parsed = JSON.parse(displayData);
      const tokens = tokenize(parsed);
      return tokens.map((token, i) => (
        <span key={i} className={colorMap[token.type]}>
          {token.value}
        </span>
      ));
    } catch {
      // Not valid JSON, render as plain text
      return <span className="text-foreground">{displayData}</span>;
    }
  }, [displayData]);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(safeData);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="relative">
      <Button
        variant="ghost"
        size={copied ? 'sm' : 'icon-sm'}
        className="absolute top-2 right-2 z-10 transition-all duration-150"
        onClick={handleCopy}
        aria-label={copied ? 'Copied to clipboard' : 'Copy to clipboard'}
      >
        {copied ? (
          <span className="inline-flex items-center gap-1">
            <CheckIcon className="size-3.5 text-emerald-400" />
            <span className="text-[10px] text-emerald-400">Copied!</span>
          </span>
        ) : (
          <CopyIcon className="size-3.5" />
        )}
      </Button>
      <pre className="overflow-auto rounded-lg bg-[var(--color-surface-raised)] p-4 text-xs font-mono leading-relaxed">
        <code>{rendered}</code>
      </pre>
      {isLarge && !showFull && (
        <div className="mt-2 text-center">
          <Button variant="ghost" size="sm" onClick={() => setShowFull(true)}>
            Show full ({(safeData.length / 1024).toFixed(0)} KB)
          </Button>
        </div>
      )}
    </div>
  );
}
