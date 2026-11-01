import { formatDate, formatRelative } from '@/lib/peer-utils';

const SOON_MS = 3 * 24 * 3600 * 1000;

/**
 * When a peer expires, highlighted within three days of `now`. `now` is the
 * time the peers were fetched, so rendering stays pure.
 */
export const PeerExpires = ({ iso, now }: { iso?: string | null; now: number }) => {
  if (!iso) return <span className="text-muted-foreground">Never</span>;
  const soon = new Date(iso).getTime() - now < SOON_MS;
  return (
    <time
      dateTime={iso}
      className={soon ? 'text-warning-foreground' : 'text-muted-foreground'}
      title={formatDate(iso)}
    >
      {formatRelative(iso)}
      {soon ? <span className="sr-only"> (soon)</span> : null}
    </time>
  );
};
