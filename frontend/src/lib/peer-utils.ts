const dateTimeFormatOptions: Intl.DateTimeFormatOptions = {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
};

export const NODES_PAGE_SIZE = 12;
export const PAGE_SIZES = [25, 50, 100, 250] as const;

export function formatBytes(n: number): string {
  if (n >= 1024 * 1024 * 1024) return (n / 1024 / 1024 / 1024).toFixed(1) + ' GB';
  if (n >= 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB';
  if (n >= 1024) return (n / 1024).toFixed(1) + ' KB';
  return n + ' B';
}

export function formatDate(iso: string | null): string {
  if (!iso) return '—';
  try {
    return new Intl.DateTimeFormat('de-DE', dateTimeFormatOptions).format(new Date(iso));
  } catch {
    return iso;
  }
}

export function formatExpires(iso: string | null | undefined): string {
  if (!iso) return 'No limit';
  try {
    return new Intl.DateTimeFormat('de-DE', dateTimeFormatOptions).format(new Date(iso));
  } catch {
    return iso;
  }
}

export function truncate(s: string, len: number): string {
  if (s.length <= len) return s;
  return s.slice(0, len) + '…';
}
