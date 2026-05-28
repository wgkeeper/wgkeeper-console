import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { AlertCircle, Clock, RefreshCw, Server, Trash2, TriangleAlert } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Skeleton } from '@/components/ui/skeleton';
import { NodeStatusBadge } from '@/components/nodes/NodeStatusBadge';
import type { NodeItem, NodeStats } from '../types';
import { NodeConfigTab } from '@/components/nodes/NodeConfigTab';
import { NodePeersTab } from '@/components/nodes/NodePeersTab';
import { formatDate } from '@/lib/peer-utils';
import { rateLimitMessage } from '@/lib/api-client';

type Props = {
  nodes: NodeItem[];
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  onReloadNodes: () => Promise<void>;
};

type TabKey = 'overview' | 'peers' | 'config';

const Stat = ({ label, children }: { label: string; children: React.ReactNode }) => (
  <div className="flex flex-col gap-1">
    <span className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
      {label}
    </span>
    <span className="font-mono text-sm">{children}</span>
  </div>
);

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
  const [deleteError, setDeleteError] = useState('');

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
    setDeleteError('');
    try {
      const res = await apiFetch(`/api/nodes/${encodeURIComponent(node.id)}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        setDeleteDialogOpen(false);
        await onReloadNodes();
        void navigate('/nodes');
        return;
      }
      if (res.status === 404) {
        // Already gone — treat as success and reconcile the list.
        setDeleteDialogOpen(false);
        await onReloadNodes();
        void navigate('/nodes');
        return;
      }
      setDeleteError(rateLimitMessage(res) ?? 'Failed to delete node. Please try again.');
    } catch {
      setDeleteError('Network error. Please try again.');
    } finally {
      setIsDeleting(false);
    }
  };

  if (!node) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-center">
        <div className="flex size-11 items-center justify-center rounded-lg border border-border bg-muted/50 text-muted-foreground">
          <Server className="size-5" />
        </div>
        <div className="flex flex-col gap-1">
          <p className="text-sm font-medium text-foreground">Node not found</p>
          <p className="text-sm text-muted-foreground">
            It may have been removed from the console.
          </p>
        </div>
        <Button variant="outline" size="sm" asChild>
          <Link to="/nodes">Back to nodes</Link>
        </Button>
      </div>
    );
  }

  const possible = stats?.peers?.possible ?? 0;
  const issued = stats?.peers?.issued ?? 0;
  const active = stats?.peers?.active ?? 0;
  const free = Math.max(0, possible - issued);
  const pct = possible > 0 ? (issued / possible) * 100 : 0;
  const pctDisplay = pct > 0 && pct < 1 ? pct.toFixed(1) : pct.toFixed(0);

  const createdAtLabel = formatDate(node.createdAt);
  const updatedAtLabel = formatDate(node.updatedAt);

  return (
    <div className="flex flex-col gap-6">
      {/* Identity header */}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex items-start gap-4">
          <div className="flex size-11 shrink-0 items-center justify-center rounded-lg border border-border bg-muted/50 text-muted-foreground">
            <Server className="size-5" />
          </div>
          <div className="flex min-w-0 flex-col gap-2">
            <div className="flex flex-wrap items-center gap-2.5">
              <h2 className="font-mono text-base font-medium text-foreground" title={node.address}>
                {node.address}
              </h2>
              <NodeStatusBadge status={node.status} />
            </div>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-xs text-muted-foreground">
              {node.version != null && (
                <Badge
                  variant={node.isOutdated ? 'warning' : 'secondary'}
                  className="gap-1 font-mono text-[11px]"
                >
                  {node.isOutdated ? <TriangleAlert /> : null}v{node.version}
                </Badge>
              )}
              <Badge variant="outline" className="font-mono text-[11px] uppercase">
                {new URL(node.address).protocol.replace(':', '')}
              </Badge>
              <span className="flex items-center gap-1">
                <Clock className="size-3" />
                Created {createdAtLabel}
              </span>
              <span className="text-border">·</span>
              <span>Updated {updatedAtLabel}</span>
            </div>
            <p className="font-mono text-xs text-muted-foreground select-all" title={node.id}>
              {node.id}
            </p>
            {node.isOutdated && node.latestVersion ? (
              <p className="flex items-center gap-2 text-sm text-warning-foreground">
                <TriangleAlert className="size-4 shrink-0" />
                <span>v{node.latestVersion} available</span>
                {node.latestVersionUrl ? (
                  <a
                    href={node.latestVersionUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="font-medium underline underline-offset-4"
                  >
                    release notes
                  </a>
                ) : null}
              </p>
            ) : null}
          </div>
        </div>
        <Button
          variant="outline"
          size="icon"
          className="size-8"
          title="Refresh status"
          aria-label="Refresh status"
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
      </div>

      <AlertDialog
        open={deleteDialogOpen}
        onOpenChange={(open) => {
          setDeleteDialogOpen(open);
          if (!open) setDeleteError('');
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete node?</AlertDialogTitle>
            <AlertDialogDescription>
              This removes the node from the console. The remote server configuration is not
              changed.
            </AlertDialogDescription>
          </AlertDialogHeader>
          {deleteError && (
            <Alert variant="destructive">
              <AlertCircle />
              <AlertDescription>{deleteError}</AlertDescription>
            </Alert>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Cancel</AlertDialogCancel>
            <Button
              variant="destructive"
              disabled={isDeleting}
              onClick={(e) => {
                e.preventDefault();
                void handleDeleteNode();
              }}
            >
              {isDeleting ? 'Deleting…' : 'Delete node'}
            </Button>
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
                  <div className="flex gap-4 border-t border-border pt-4">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-4 w-28" />
                    <Skeleton className="h-4 w-24" />
                  </div>
                </div>
              ) : statsError ? (
                <p className="text-sm text-muted-foreground">{statsError}</p>
              ) : stats ? (
                <>
                  <div className="flex flex-col gap-1.5">
                    <div className="flex flex-wrap items-baseline justify-between gap-2 text-sm">
                      <span className="font-medium">Peer capacity</span>
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
                        className="h-full rounded-full bg-primary transition-[width] duration-300 ease-out"
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
                  <div className="grid grid-cols-2 gap-x-4 gap-y-4 sm:grid-cols-3">
                    {(stats.wireguard?.subnets?.length ?? 0) > 0 && (
                      <Stat
                        label={`Subnet${(stats.wireguard?.subnets?.length ?? 0) > 1 ? 's' : ''}`}
                      >
                        {stats.wireguard?.subnets?.join(', ')}
                      </Stat>
                    )}
                    {(stats.wireguard?.serverIps?.length ?? 0) > 0 && (
                      <Stat label="Server IPs">{stats.wireguard?.serverIps?.join(', ')}</Stat>
                    )}
                    {(stats.wireguard?.addressFamilies?.length ?? 0) > 0 && (
                      <Stat label="Families">{stats.wireguard?.addressFamilies?.join(', ')}</Stat>
                    )}
                    {stats.wireguard?.listenPort != null && (
                      <Stat label="WireGuard port">{stats.wireguard.listenPort}</Stat>
                    )}
                    {stats.service?.name != null && (
                      <Stat label="Service">
                        {stats.service.name}
                        {stats.service.version != null && (
                          <span className="text-muted-foreground"> v{stats.service.version}</span>
                        )}
                      </Stat>
                    )}
                  </div>
                </>
              ) : null}
            </CardContent>
          </Card>

          <Card className="border-destructive/30">
            <CardHeader>
              <CardTitle className="text-destructive">Danger zone</CardTitle>
              <CardDescription>
                Removes the node from the console. The remote server configuration is not changed.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Button
                variant="destructive"
                size="sm"
                disabled={isDeleting}
                onClick={() => setDeleteDialogOpen(true)}
              >
                <Trash2 className="size-3.5" />
                Delete node
              </Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="peers">
          <NodePeersTab nodeId={node.id} apiFetch={apiFetch} refreshTrigger={refreshPeersTrigger} />
        </TabsContent>

        <TabsContent value="config">
          <NodeConfigTab
            nodeId={node.id}
            apiFetch={apiFetch}
            onPeersChanged={handlePeersChanged}
            supportedAddressFamilies={stats?.wireguard?.addressFamilies}
            isStatsLoading={statsLoading && !stats}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
};
