import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

type Tone = 'success' | 'offline' | 'warning' | 'muted';

// Offline is a hollow ring, not just a red dot, so it reads without color.
const dotClass: Record<Tone, string> = {
  success: 'size-1.5 bg-success',
  offline: 'size-2 border-[1.5px] border-destructive',
  warning: 'size-1.5 bg-warning',
  muted: 'size-1.5 bg-muted-foreground/50',
};

/** A status mark that sits beside a word, so state never relies on color alone. */
export const StatusDot = ({ tone, className }: { tone: Tone; className?: string }) => (
  <span
    aria-hidden
    className={cn('inline-block shrink-0 rounded-full', dotClass[tone], className)}
  />
);

const toneFor = (status: string): Tone =>
  status === 'online' ? 'success' : status === 'offline' ? 'offline' : 'warning';

export const NodeStatusBadge = ({ status, className }: { status: string; className?: string }) => (
  <Badge variant="outline" className={cn('gap-1.5 capitalize', className)}>
    <StatusDot tone={toneFor(status)} />
    {status}
  </Badge>
);

export const PeerStatusBadge = ({ active, className }: { active: boolean; className?: string }) => (
  <Badge variant="outline" className={cn('gap-1.5', className)}>
    <StatusDot tone={active ? 'success' : 'muted'} />
    {active ? 'Active' : 'Inactive'}
  </Badge>
);
