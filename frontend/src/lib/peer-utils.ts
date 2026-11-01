export const NODES_PAGE_SIZE = 12;
export const PAGE_SIZES = [25, 50, 100, 250] as const;

export function formatBytes(n: number): string {
  if (n >= 1024 * 1024 * 1024) return (n / 1024 / 1024 / 1024).toFixed(1) + ' GB';
  if (n >= 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB';
  if (n >= 1024) return (n / 1024).toFixed(1) + ' KB';
  return n + ' B';
}

// Built once: a 250-row table formats several dates per row on every render.
// The browser's locale decides the order and separators.
const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

/** "1 Nov 2026, 17:36" in the browser's locale; "—" for a missing date. */
export function formatDate(iso: string | null | undefined): string {
  if (!iso) return '—';
  try {
    return dateFormat.format(new Date(iso));
  } catch {
    return iso;
  }
}

const relativeUnits: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['week', 7 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
];

const relativeFormat = new Intl.RelativeTimeFormat('en', { numeric: 'auto', style: 'short' });

/** "3 min. ago", "in 2 days"; "—" for a missing date. */
export function formatRelative(iso: string | null | undefined, now = Date.now()): string {
  if (!iso) return '—';
  const time = new Date(iso).getTime();
  if (Number.isNaN(time)) return iso;
  const seconds = Math.round((time - now) / 1000);
  for (const [unit, size] of relativeUnits) {
    if (Math.abs(seconds) >= size) return relativeFormat.format(Math.trunc(seconds / size), unit);
  }
  return Math.abs(seconds) < 10 ? 'just now' : relativeFormat.format(seconds, 'second');
}

/** "1 peer", "3 peers". */
export const plural = (count: number, one: string, many = `${one}s`) =>
  `${count} ${count === 1 ? one : many}`;
