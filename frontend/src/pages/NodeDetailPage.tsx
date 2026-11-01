import { Activity, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  ArrowUpCircleIcon,
  CircleAlertIcon,
  CopyIcon,
  ExternalLinkIcon,
  FilePlus2Icon,
  MoreHorizontalIcon,
  RefreshCwIcon,
  ServerOffIcon,
  Trash2Icon,
} from 'lucide-react';
import { toast } from 'sonner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { CopyButton, copyToClipboard } from '@/components/CopyButton';
import { DeleteNodeDialog } from '@/components/nodes/DeleteNodeDialog';
import { NodeConfigTab } from '@/components/nodes/NodeConfigTab';
import { NodePeersTab } from '@/components/nodes/NodePeersTab';
import { NodeStatusBadge, StatusDot } from '@/components/nodes/NodeStatusBadge';
import { nodeHost, nodeScheme } from '@/lib/node-utils';
import { formatDate, formatRelative } from '@/lib/peer-utils';
import { useSearchParam } from '@/hooks/use-search-param';
import { cn } from '@/lib/utils';
import type { NodeItem, NodeStats } from '../types';

type Props = {
  nodeId: string;
  /** The node as the list knows it, for an instant first render. */
  initialNode: NodeItem | null;
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  onReloadNodes: () => Promise<boolean>;
};

type TabKey = 'overview' | 'peers' | 'config';

export const NodeDetailPage = ({ nodeId, initialNode, apiFetch, onReloadNodes }: Props) => {
  const navigate = useNavigate();
  const [tabParam, setTabParam] = useSearchParam('tab', 'overview');
  const tab: TabKey = tabParam === 'peers' || tabParam === 'config' ? tabParam : 'overview';
  const setTab = (value: string) => setTabParam(value);

  // The page fetches its own node: the list in App holds only one page of
  // nodes, so a deep link to any other node must not depend on it.
  const [node, setNode] = useState<NodeItem | null>(initialNode);
  const [nodeState, setNodeState] = useState<'loading' | 'ready' | 'missing' | 'error'>(
    initialNode ? 'ready' : 'loading',
  );
  const [refreshNodeTrigger, setRefreshNodeTrigger] = useState(0);
  const [stats, setStats] = useState<NodeStats | null>(null);
  const [statsLoading, setStatsLoading] = useState(false);
  const [statsError, setStatsError] = useState('');
  const [refreshStatsTrigger, setRefreshStatsTrigger] = useState(0);
  const [refreshPeersTrigger, setRefreshPeersTrigger] = useState(0);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    const load = async () => {
      try {
        const res = await apiFetch(`/api/nodes/${encodeURIComponent(nodeId)}`, {
          signal: controller.signal,
        });
        if (res.status === 404) {
          setNode(null);
          setNodeState('missing');
          return;
        }
        if (!res.ok) throw new Error('Node unavailable');
        setNode((await res.json()) as NodeItem);
        setNodeState('ready');
      } catch (err) {
        if ((err as Error).name === 'AbortError') return;
        // Keep showing the node from the list if there is one.
        setNodeState((state) => (state === 'ready' ? state : 'error'));
      }
    };
    void load();
    return () => controller.abort();
    // refreshNodeTrigger isn't read inside: bumping it is how callers ask for a reload.
    // oxlint-disable-next-line react/exhaustive-effect-dependencies
  }, [nodeId, apiFetch, refreshNodeTrigger]);

  // Keyed on the route ID, so stats load in parallel with the node itself.
  useEffect(() => {
    const controller = new AbortController();
    setStatsLoading(true);
    setStatsError('');
    const load = async () => {
      try {
        const res = await apiFetch(`/api/nodes/${encodeURIComponent(nodeId)}/stats`, {
          signal: controller.signal,
        });
        if (!res.ok) throw new Error('Stats unavailable');
        setStats((await res.json()) as NodeStats);
        setStatsError('');
      } catch (err) {
        if ((err as Error).name === 'AbortError') return;
        setStats(null);
        setStatsError('The node did not return its stats. It may be offline.');
      } finally {
        if (!controller.signal.aborted) setStatsLoading(false);
      }
    };
    void load();
    return () => controller.abort();
    // refreshStatsTrigger isn't read inside: bumping it is how callers ask for a reload.
    // oxlint-disable-next-line react/exhaustive-effect-dependencies
  }, [nodeId, apiFetch, refreshStatsTrigger]);

  const handlePeersChanged = () => {
    setRefreshStatsTrigger((s) => s + 1);
    setRefreshPeersTrigger((p) => p + 1);
  };

  const handleRefresh = async () => {
    setIsRefreshing(true);
    let refreshed = false;
    try {
      refreshed = (await apiFetch('/api/nodes/refresh', { method: 'POST' })).ok;
    } catch {
      // reported below
    }
    setRefreshNodeTrigger((n) => n + 1);
    setRefreshStatsTrigger((s) => s + 1);
    setRefreshPeersTrigger((p) => p + 1);
    void onReloadNodes();
    if (refreshed) toast.success('Node refreshed');
    else toast.error('The node could not be refreshed. Showing the last known state.');
    setIsRefreshing(false);
  };

  if (!node) {
    if (nodeState === 'loading') return <DetailSkeleton />;
    return (
      <Empty className="border border-dashed">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <ServerOffIcon />
          </EmptyMedia>
          <EmptyTitle>{nodeState === 'missing' ? 'Node not found' : 'Node unavailable'}</EmptyTitle>
          <EmptyDescription>
            {nodeState === 'missing'
              ? 'It may have been removed from the console.'
              : 'The console could not load this node. Try again.'}
          </EmptyDescription>
        </EmptyHeader>
        <EmptyContent className="flex-row justify-center">
          {nodeState === 'error' ? (
            <Button size="sm" onClick={() => setRefreshNodeTrigger((n) => n + 1)}>
              Try again
            </Button>
          ) : null}
          <Button variant="outline" size="sm" asChild>
            <Link to="/nodes">Back to nodes</Link>
          </Button>
        </EmptyContent>
      </Empty>
    );
  }

  const scheme = nodeScheme(node.address);
  const issued = stats?.peers?.issued;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-start justify-between gap-x-6 gap-y-4">
        <div className="flex min-w-0 flex-col gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="truncate font-mono text-xl font-semibold tracking-tight">
              {nodeHost(node.address)}
            </h1>
            <NodeStatusBadge status={node.status} />
            {node.version ? (
              <Badge variant="secondary" className="font-mono">
                v{node.version}
              </Badge>
            ) : null}
            {scheme ? (
              <Badge variant="outline" className="font-mono text-muted-foreground">
                {scheme}
              </Badge>
            ) : null}
          </div>
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
            <span className="flex items-center gap-0.5">
              <span className="font-mono text-xs">{node.id}</span>
              <CopyButton value={node.id} label="Copy node ID" className="size-6" />
            </span>
            <time dateTime={node.createdAt} title={formatDate(node.createdAt)}>
              Added {formatRelative(node.createdAt)}
            </time>
            <time dateTime={node.updatedAt} title={formatDate(node.updatedAt)}>
              Checked {formatRelative(node.updatedAt)}
            </time>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button variant="outline" disabled={isRefreshing} onClick={() => void handleRefresh()}>
            <RefreshCwIcon className={cn(isRefreshing && 'animate-spin')} />
            Refresh
          </Button>
          <Button onClick={() => setTab('config')}>
            <FilePlus2Icon />
            New config
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="icon" aria-label="More actions">
                <MoreHorizontalIcon />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-52">
              <DropdownMenuItem
                onSelect={() => void copyToClipboard(node.address, 'Address copied')}
              >
                <CopyIcon />
                Copy address
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => void copyToClipboard(node.id, 'Node ID copied')}>
                <CopyIcon />
                Copy node ID
              </DropdownMenuItem>
              {node.latestVersionUrl ? (
                <DropdownMenuItem asChild>
                  <a href={node.latestVersionUrl} target="_blank" rel="noreferrer">
                    <ExternalLinkIcon />
                    Release notes
                  </a>
                </DropdownMenuItem>
              ) : null}
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onSelect={() => setDeleteOpen(true)}>
                <Trash2Icon />
                Remove node
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {node.isOutdated && node.latestVersion ? (
        <Alert className="[&>svg]:text-warning-foreground">
          <ArrowUpCircleIcon />
          <AlertTitle>wgkeeper-node v{node.latestVersion} is available</AlertTitle>
          <AlertDescription>
            <p>
              This node runs v{node.version}.{' '}
              {node.latestVersionUrl ? (
                <a
                  href={node.latestVersionUrl}
                  target="_blank"
                  rel="noreferrer"
                  className="font-medium text-foreground underline underline-offset-4"
                >
                  Read the release notes
                </a>
              ) : null}
            </p>
          </AlertDescription>
        </Alert>
      ) : null}

      <Tabs value={tab} onValueChange={setTab} className="gap-6">
        <TabsList variant="line" className="w-full justify-start border-b pb-[5px]">
          <TabsTrigger value="overview" className="flex-none px-3">
            Overview
          </TabsTrigger>
          <TabsTrigger value="peers" className="flex-none px-3">
            Peers
            {issued === undefined ? null : (
              <Badge variant="secondary" className="h-5 min-w-5 px-1.5 tabular-nums">
                {issued}
              </Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="config" className="flex-none px-3">
            Config
          </TabsTrigger>
        </TabsList>

        {/* Panels stay mounted under <Activity>: switching tabs keeps the peers
            page and filters, and a config generated a moment ago with its
            private key, instead of throwing them away. */}
        <TabsContent value="overview" forceMount className="data-[state=inactive]:hidden">
          <Activity mode={tab === 'overview' ? 'visible' : 'hidden'}>
            <Overview
              stats={stats}
              loading={statsLoading && !stats}
              error={statsError}
              onRetry={() => setRefreshStatsTrigger((s) => s + 1)}
            />
          </Activity>
        </TabsContent>
        <TabsContent value="peers" forceMount className="data-[state=inactive]:hidden">
          <Activity mode={tab === 'peers' ? 'visible' : 'hidden'}>
            <NodePeersTab
              nodeId={node.id}
              apiFetch={apiFetch}
              refreshTrigger={refreshPeersTrigger}
              onPeersChanged={() => setRefreshStatsTrigger((s) => s + 1)}
              onNewConfig={() => setTab('config')}
            />
          </Activity>
        </TabsContent>
        <TabsContent value="config" forceMount className="data-[state=inactive]:hidden">
          <Activity mode={tab === 'config' ? 'visible' : 'hidden'}>
            <NodeConfigTab
              nodeId={node.id}
              apiFetch={apiFetch}
              onPeersChanged={handlePeersChanged}
              onShowPeers={() => setTab('peers')}
              supportedAddressFamilies={stats?.wireguard?.addressFamilies}
              isStatsLoading={statsLoading && !stats}
            />
          </Activity>
        </TabsContent>
      </Tabs>

      <DeleteNodeDialog
        node={deleteOpen ? node : null}
        onOpenChange={setDeleteOpen}
        apiFetch={apiFetch}
        onDeleted={async () => {
          await onReloadNodes();
          void navigate('/nodes');
        }}
      />
    </div>
  );
};

const DetailSkeleton = () => (
  <div className="flex flex-col gap-6" aria-busy="true">
    <div className="flex flex-col gap-2">
      <Skeleton className="h-7 w-64" />
      <Skeleton className="h-4 w-96 max-w-full" />
    </div>
    <Skeleton className="h-9 w-72" />
    <div className="grid gap-4 lg:grid-cols-3">
      <Skeleton className="h-48 rounded-xl lg:col-span-2" />
      <Skeleton className="h-48 rounded-xl" />
    </div>
  </div>
);

const Overview = ({
  stats,
  loading,
  error,
  onRetry,
}: {
  stats: NodeStats | null;
  loading: boolean;
  error: string;
  onRetry: () => void;
}) => {
  if (loading) {
    return (
      <div className="grid gap-4 lg:grid-cols-3">
        <Skeleton className="h-48 rounded-xl lg:col-span-2" />
        <Skeleton className="h-48 rounded-xl" />
        <Skeleton className="h-40 rounded-xl lg:col-span-3" />
      </div>
    );
  }
  if (error || !stats) {
    return (
      <Alert variant="destructive">
        <CircleAlertIcon />
        <AlertTitle>Stats unavailable</AlertTitle>
        <AlertDescription>
          <p>{error || 'The node returned no stats.'}</p>
          <Button variant="outline" size="sm" className="mt-2" onClick={onRetry}>
            Try again
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  const possible = stats.peers?.possible ?? 0;
  const issued = stats.peers?.issued ?? 0;
  const active = stats.peers?.active ?? 0;
  const free = Math.max(0, possible - issued);
  const pct = possible > 0 ? (issued / possible) * 100 : 0;
  const wg = stats.wireguard;

  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle>
            <h2>Peer capacity</h2>
          </CardTitle>
          <CardDescription>Addresses issued from the node’s subnets.</CardDescription>
          <CardAction className="text-sm text-muted-foreground tabular-nums">
            {pct > 0 && pct < 1 ? pct.toFixed(1) : pct.toFixed(0)}% used
          </CardAction>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex items-baseline gap-1.5">
            <span className="text-3xl font-semibold tracking-tight tabular-nums">{issued}</span>
            <span className="text-muted-foreground tabular-nums">/ {possible} issued</span>
          </div>
          <Progress value={pct} aria-label="Peer capacity used" />
          <dl className="grid grid-cols-3 gap-4 text-sm">
            <Metric label="Active now" value={active} dot="success" />
            <Metric label="Issued" value={issued} dot="muted" />
            <Metric label="Free" value={free} />
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>Service</h2>
          </CardTitle>
          <CardDescription>What answers on the node API.</CardDescription>
        </CardHeader>
        <CardContent>
          <dl className="flex flex-col gap-3 text-sm">
            <Row label="Name" value={stats.service?.name} />
            <Row
              label="Version"
              value={stats.service?.version ? `v${stats.service.version}` : undefined}
            />
            <Row
              label="Running since"
              value={stats.startedAt ? formatDate(stats.startedAt) : undefined}
              title={stats.startedAt ? formatRelative(stats.startedAt) : undefined}
              mono={false}
            />
          </dl>
        </CardContent>
      </Card>

      <Card className="lg:col-span-3">
        <CardHeader>
          <CardTitle>
            <h2>WireGuard</h2>
          </CardTitle>
          <CardDescription>The interface peers connect to.</CardDescription>
        </CardHeader>
        <CardContent>
          <dl className="grid gap-x-8 gap-y-3 text-sm sm:grid-cols-2 lg:grid-cols-3">
            <Row label="Interface" value={wg?.interface} />
            <Row label="Listen port" value={wg?.listenPort?.toString()} />
            <Row label="Address families" value={wg?.addressFamilies?.join(', ')} />
            <Row label="Subnets" value={wg?.subnets?.join(', ')} />
            <Row label="Server IPs" value={wg?.serverIps?.join(', ')} />
          </dl>
        </CardContent>
      </Card>
    </div>
  );
};

const Metric = ({
  label,
  value,
  dot,
}: {
  label: string;
  value: number;
  dot?: 'success' | 'muted';
}) => (
  <div className="flex flex-col gap-0.5">
    <dt className="flex items-center gap-1.5 text-muted-foreground">
      {dot ? <StatusDot tone={dot} /> : null}
      {label}
    </dt>
    <dd className="font-medium tabular-nums">{value}</dd>
  </div>
);

const Row = ({
  label,
  value,
  title,
  mono = true,
}: {
  label: string;
  value?: string;
  title?: string;
  mono?: boolean;
}) => (
  <div className="flex items-baseline justify-between gap-4 border-b border-dashed pb-3 last:border-0 last:pb-0 sm:block sm:border-0 sm:pb-0">
    <dt className="text-muted-foreground">{label}</dt>
    <dd className={cn('truncate sm:mt-0.5', mono && 'font-mono text-[0.8125rem]')} title={title}>
      {value ?? '—'}
    </dd>
  </div>
);
