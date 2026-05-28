import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

type Props = {
  status: string;
  className?: string;
};

const variantFor = (status: string) =>
  status === 'online' ? 'success' : status === 'offline' ? 'destructive' : 'warning';

const dotFor = (status: string) =>
  status === 'online' ? 'bg-success' : status === 'offline' ? 'bg-destructive' : 'bg-warning';

/**
 * Status pill that pairs a colored dot with the status word, so node state is
 * legible without relying on color alone.
 */
export const NodeStatusBadge = ({ status, className }: Props) => (
  <Badge variant={variantFor(status)} className={cn('capitalize', className)}>
    <span className={cn('size-1.5 rounded-full', dotFor(status))} aria-hidden />
    {status}
  </Badge>
);
