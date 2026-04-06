import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { ChevronRight, RefreshCw, Server, Trash2, Clock, TriangleAlert } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Skeleton } from '@/components/ui/skeleton';
import type { NodeItem, NodeStats } from '../types';
import { NodeConfigTab } from '@/components/nodes/NodeConfigTab';
import { NodePeersTab } from '@/components/nodes/NodePeersTab';
import { formatDate } from '@/lib/peer-utils';

type Props = {
  nodes: NodeItem[];
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  onReloadNodes: () => Promise<void>;
};

type TabKey = 'overview' | 'peers' | 'config';

export const NodeDetailPage = ({ nodes, apiFetch, onReloadNodes }: Props) => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const tabParam = searchParams.get('tab');
  const defaultTab: TabKey = tabParam === 'peers' || tabParam === 'config' ? tabParam : 'overview';

  const node = id != null ? (nodes.find((n) => n.id === id) ?? null) : null;

  const onTabChange = (value: string) => {
    setSearchParams(value === 'overview' ? {} : { tab: value }, { replace: true });
  };

  const [stats, setStats] = useState<NodeStats | null>(null);
  const [statsLoading, setStatsLoading] = useState(false);
  const [statsError, setStatsError] = useState('');
  const [refreshStatsTrigger, setRefreshStatsTrigger] = useState(0);
  const [refreshPeersTrigger, setRefreshPeersTrigger] = useState(0);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isRefreshingStatus, setIsRefreshingStatus] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const nodeId = node?.id;
  useEffect(() => {
    if (!nodeId) return;
    const controller = new AbortController();
    setStatsLoading(true);
    setStatsError('');
    apiFetch(`/api/nodes/${encodeURIComponent(nodeId)}/stats`, { signal: controller.signal })
      .then((res) => {
        if (!res.ok) throw new Error('Stats unavailable');
        return res.json() as Promise<NodeStats>;
      })
      .then((data) => {
        setStats(data);
        setStatsError('');
      })
      .catch((err: unknown) => {
        if ((err as Error).name === 'AbortError') return;
        setStats(null);
        setStatsError('Stats unavailable');
      })
      .finally(() => {
        if (!controller.signal.aborted) setStatsLoading(false);
      });
    return () => controller.abort();
  }, [nodeId, apiFetch, refreshStatsTrigger]);

  const handlePeersChanged = () => {
    setRefreshStatsTrigger((s) => s + 1);
    setRefreshPeersTrigger((p) => p + 1);
  };

  const handleDeleteNode = async () => {
    if (!node) return;
    setIsDeleting(true);
    try {
      const res = await apiFetch(`/api/nodes/${encodeURIComponent(node.id)}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        await onReloadNodes();
        void navigate('/nodes');
      }
    } catch {
      // best-effort
    } finally {
      setIsDeleting(false);
    }
  };

  if (!node) {
    return (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">Node not found.</p>
        <Link to="/nodes" className="text-sm text-primary hover:underline">
          ← Back to nodes
        </Link>
      </div>
    );
  }

  const possible = stats?.peers?.possible ?? 0;
  const issued = stats?.peers?.issued ?? 0;
  const active = stats?.peers?.active ?? 0;
  const free = Math.max(0, possible - issued);
  const pct = possible > 0 ? (issued / possible) * 100 : 0;
  const pctDisplay = pct > 0 && pct < 1 ? pct.toFixed(1) : pct.toFixed(0);

  const nodeAddress = node.address;
  const hostPortLabel = nodeAddress;
  const createdAtLabel = formatDate(node.createdAt);
  const updatedAtLabel = formatDate(node.updatedAt);

  return (
    <div className="flex flex-col gap-6">
      <nav className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link to="/nodes" className="transition-colors hover:text-foreground">
          Nodes
        </Link>
        <ChevronRight className="size-3.5" />
        <span className="truncate font-mono text-foreground" title={nodeAddress}>
          {hostPortLabel}
        </span>
      </nav>

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex items-start gap-4">
          <div className="flex size-11 flex-shrink-0 items-center justify-center rounded-xl bg-muted">
            <Server className="size-5 text-muted-foreground" />
          </div>
          <div className="flex flex-col gap-1.5">
            <div className="flex flex-wrap items-center gap-2">
              <Badge
                variant="secondary"
                className={`gap-1.5 ${
                  node.status === 'online'
                    ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300'
                    : node.status === 'offline'
                      ? 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300'
                      : 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300'
                }`}
              >
                <span
                  className={`size-1.5 rounded-full ${
                    node.status === 'online'
                      ? 'bg-emerald-500'
                      : node.status === 'offline'
                        ? 'bg-red-500'
                        : 'bg-amber-500'
                  }`}
                />
                {node.status}
              </Badge>
              {node.version != null && (
                <Badge
                  variant="outline"
                  className={`font-mono text-xs ${
                    node.isOutdated
                      ? 'border-foreground/50 bg-foreground/[0.06] text-foreground shadow-[inset_0_0_0_1px_hsl(var(--foreground)/0.08)] dark:bg-foreground/[0.08]'
                      : ''
                  }`}
                >
                  v{node.version}
                </Badge>
              )}
              <Badge variant="secondary" className="font-mono text-xs uppercase">
                {new URL(node.address).protocol.replace(':', '')}
              </Badge>
            </div>
            <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
              <span className="flex items-center gap-1">
                <Clock className="size-3" />
                Created {createdAtLabel}
              </span>
              <span className="text-border">·</span>
              <span>Updated {updatedAtLabel}</span>
            </div>
            <p className="font-mono text-xs text-foreground/60 select-all" title={node.id}>
              {node.id}
            </p>
            {node.isOutdated && node.latestVersion ? (
              <p className="flex items-center gap-2 text-sm text-foreground">
                <TriangleAlert className="size-4 text-amber-500" />
                <span>v{node.latestVersion} available</span>
                {node.latestVersionUrl ? (
                  <>
                    <span className="text-border"> · </span>
                    <a
                      href={node.latestVersionUrl}
                      target="_blank"
                      rel="noreferrer"
                      className="underline underline-offset-4"
                    >
                      open release
                    </a>
                  </>
                ) : null}
              </p>
            ) : null}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            title="Refresh status"
            disabled={isRefreshingStatus}
            onClick={async () => {
              setIsRefreshingStatus(true);
              try {
                await apiFetch('/api/nodes/refresh', { method: 'POST' });
                await onReloadNodes();
                setRefreshStatsTrigger((s) => s + 1);
              } catch {
                // best-effort
              } finally {
                setIsRefreshingStatus(false);
              }
            }}
          >
            <RefreshCw className={`size-4 ${isRefreshingStatus ? 'animate-spin' : ''}`} />
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="gap-1.5"
            disabled={isDeleting}
            onClick={() => setDeleteDialogOpen(true)}
          >
            <Trash2 className="size-3.5" />
            Delete
          </Button>
        </div>
      </div>

      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete node?</AlertDialogTitle>
            <AlertDialogDescription>
              This will remove the node from the console but will not change the remote server
              configuration.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={handleDeleteNode}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Tabs value={defaultTab} onValueChange={onTabChange} className="flex flex-col gap-4">
        <TabsList className="grid w-full max-w-md grid-cols-3">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="peers">Peers</TabsTrigger>
          <TabsTrigger value="config">Config</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <CardTitle>Node stats</CardTitle>
              <CardDescription>Capacity and network info for this node.</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              {statsLoading && !stats ? (
                <div className="flex flex-col gap-4">
                  <div className="flex flex-col gap-1.5">
                    <div className="flex items-baseline justify-between">
                      <Skeleton className="h-4 w-24" />
                      <Skeleton className="h-4 w-52" />
                    </div>
                    <Skeleton className="h-2 w-full rounded-full" />
                    <Skeleton className="h-3 w-16" />
                  </div>
                  <div className="flex gap-4 border-t pt-4">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-4 w-28" />
                    <Skeleton className="h-4 w-24" />
                  </div>
                </div>
              ) : statsError ? (
                <p className="text-sm text-destructive">{statsError}</p>
              ) : stats ? (
                <>
                  <div className="flex flex-col gap-1.5">
                    <div className="flex flex-wrap items-baseline justify-between gap-2 text-sm">
                      <span className="font-medium text-muted-foreground">Peer capacity</span>
                      <span className="tabular-nums">
                        <span className="font-medium">{active}</span>
                        <span className="text-muted-foreground"> active</span>
                        {' · '}
                        <span className="font-medium">{issued}</span>
                        <span className="text-muted-foreground"> issued</span>
                        {' / '}
                        <span className="font-medium">{possible}</span>
                        <span className="text-muted-foreground"> possible</span>
                        {possible > 0 && (
                          <span className="ml-1 text-muted-foreground">({pctDisplay}% used)</span>
                        )}
                      </span>
                    </div>
                    <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                      <div
                        className="h-full rounded-full bg-primary transition-all"
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                    {possible > 0 && (
                      <p className="text-xs text-muted-foreground">
                        {free} slot{free !== 1 ? 's' : ''} free
                      </p>
                    )}
                  </div>
                  <Separator />
                  <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                    {(stats.wireguard?.subnets?.length ?? 0) > 0 && (
                      <div className="flex flex-col gap-0.5">
                        <span className="text-[10px] tracking-wide text-muted-foreground uppercase">
                          Subnet{(stats.wireguard?.subnets?.length ?? 0) > 1 ? 's' : ''}
                        </span>
                        <span className="font-mono text-sm">
                          {stats.wireguard?.subnets?.join(', ')}
                        </span>
                      </div>
                    )}
                    {(stats.wireguard?.serverIps?.length ?? 0) > 0 && (
                      <div className="flex flex-col gap-0.5">
                        <span className="text-[10px] tracking-wide text-muted-foreground uppercase">
                          Server IPs
                        </span>
                        <span className="font-mono text-sm">
                          {stats.wireguard?.serverIps?.join(', ')}
                        </span>
                      </div>
                    )}
                    {(stats.wireguard?.addressFamilies?.length ?? 0) > 0 && (
                      <div className="flex flex-col gap-0.5">
                        <span className="text-[10px] tracking-wide text-muted-foreground uppercase">
                          Families
                        </span>
                        <span className="font-mono text-sm">
                          {stats.wireguard?.addressFamilies?.join(', ')}
                        </span>
                      </div>
                    )}
                    {stats.wireguard?.listenPort != null && (
                      <div className="flex flex-col gap-0.5">
                        <span className="text-[10px] tracking-wide text-muted-foreground uppercase">
                          Listen port
                        </span>
                        <span className="font-mono text-sm">{stats.wireguard.listenPort}</span>
                      </div>
                    )}
                    {stats.service?.name != null && (
                      <div className="flex flex-col gap-0.5">
                        <span className="text-[10px] tracking-wide text-muted-foreground uppercase">
                          Service
                        </span>
                        <span className="font-mono text-sm">
                          {stats.service.name}
                          {stats.service.version != null && (
                            <span className="text-muted-foreground"> v{stats.service.version}</span>
                          )}
                        </span>
                      </div>
                    )}
                  </div>
                </>
              ) : null}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="peers">
          <NodePeersTab nodeId={node.id} apiFetch={apiFetch} refreshTrigger={refreshPeersTrigger} />
        </TabsContent>

        <TabsContent value="config">
          <NodeConfigTab nodeId={node.id} apiFetch={apiFetch} onPeersChanged={handlePeersChanged} />
        </TabsContent>
      </Tabs>
    </div>
  );
};
