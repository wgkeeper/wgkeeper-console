import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  AlertCircle,
  CheckCircle2,
  ChevronRight,
  RefreshCw,
  Server,
  TriangleAlert,
} from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { NodeStatusBadge } from '@/components/nodes/NodeStatusBadge';
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination';
import type { NodeCheckResult, NodeItem } from '../types';
import { NODES_PAGE_SIZE } from '@/lib/peer-utils';
import { rateLimitMessage } from '@/lib/api-client';

type Props = {
  nodes: NodeItem[];
  nodesPage: number;
  nodesTotalPages: number;
  nodesTotal: number;
  nodesLoading?: boolean;
  nodesError?: string | null;
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  onReloadNodes: (page?: number) => Promise<void>;
};

export const NodesPage = ({
  nodes,
  nodesPage,
  nodesTotalPages,
  nodesTotal,
  nodesLoading,
  nodesError,
  apiFetch,
  onReloadNodes,
}: Props) => {
  const [nodeForm, setNodeForm] = useState({
    address: '',
    apiKey: '',
  });
  const [nodeCheck, setNodeCheck] = useState<NodeCheckResult | null>(null);
  const [nodeCheckError, setNodeCheckError] = useState('');
  const [isCheckingNode, setIsCheckingNode] = useState(false);
  const [isCreatingNode, setIsCreatingNode] = useState(false);
  const [isRefreshingNodes, setIsRefreshingNodes] = useState(false);

  const pageLabel = useMemo(() => {
    const from = (nodesPage - 1) * NODES_PAGE_SIZE + 1;
    const to = Math.min(nodesPage * NODES_PAGE_SIZE, nodesTotal);
    return nodesTotal > 0 ? `${from}–${to} of ${nodesTotal}` : '';
  }, [nodesPage, nodesTotal]);
  const statusSummary = useMemo(() => {
    const online = nodes.filter((n) => n.status === 'online').length;
    const offline = nodes.filter((n) => n.status === 'offline').length;
    const other = nodes.length - online - offline;
    const outdated = nodes.filter((n) => n.isOutdated).length;
    return { online, offline, other, outdated };
  }, [nodes]);
  const latestRelease = useMemo(() => {
    const latestVersion = nodes.find((node) => node.latestVersion)?.latestVersion ?? null;
    const latestVersionUrl = nodes.find((node) => node.latestVersionUrl)?.latestVersionUrl ?? null;
    return latestVersion ? { version: latestVersion, url: latestVersionUrl } : null;
  }, [nodes]);

  const updateNodeForm = (updater: React.SetStateAction<typeof nodeForm>) => {
    setNodeForm(updater);
    setNodeCheck(null);
    setNodeCheckError('');
  };

  const handleCreateNode = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setNodeCheckError('');

    if (!nodeCheck?.ok) {
      setNodeCheckError('Check the node first.');
      return;
    }

    const address = nodeForm.address.trim();
    const apiKey = nodeForm.apiKey.trim();

    if (!address || !apiKey) {
      setNodeCheckError('Fill in address and X_API_KEY.');
      return;
    }

    if (nodeCheck.address !== address) {
      setNodeCheckError('Check is outdated, please re-check the node.');
      return;
    }

    setIsCreatingNode(true);
    const res = await apiFetch('/api/nodes', {
      method: 'POST',
      body: JSON.stringify({ address, apiKey }),
    });

    if (res.ok) {
      setNodeForm({ address: '', apiKey: '' });
      setNodeCheck(null);
      await onReloadNodes(1);
    } else {
      setNodeCheck(null);
      let message = rateLimitMessage(res) ?? 'Failed to add node. Please try again.';
      if (!rateLimitMessage(res)) {
        try {
          const body = (await res.json()) as { error?: string };
          if (body?.error === 'node_exists' || res.status === 409) {
            message = 'A node with this address already exists.';
          } else if (body?.error === 'invalid_input') {
            message = 'Address or X_API_KEY is invalid.';
          }
        } catch {
          if (res.status === 409) {
            message = 'A node with this address already exists.';
          }
        }
      }
      setNodeCheckError(message);
    }
    setIsCreatingNode(false);
  };

  const handleCheckNode = async () => {
    setNodeCheckError('');
    const address = nodeForm.address.trim();
    const apiKey = nodeForm.apiKey.trim();

    if (!address || !apiKey) {
      setNodeCheckError('Fill in address and X_API_KEY.');
      return;
    }

    setIsCheckingNode(true);
    const res = await apiFetch('/api/nodes/check', {
      method: 'POST',
      body: JSON.stringify({ address, apiKey }),
    });

    if (!res.ok) {
      setNodeCheck(null);
      let errorMessage = rateLimitMessage(res) ?? 'Node is not responding.';
      if (!rateLimitMessage(res)) {
        try {
          const errorBody = (await res.json()) as { error?: string };
          if (errorBody?.error === 'invalid_api_key') {
            errorMessage = 'Invalid X_API_KEY.';
          }
        } catch {
          // ignore parse error, fall back to default message
        }
      }
      setNodeCheckError(errorMessage);
      setIsCheckingNode(false);
      return;
    }

    const data = (await res.json()) as Omit<NodeCheckResult, 'address'>;
    setNodeCheck({ ...data, address });
    if (!data.ok) {
      setNodeCheckError('Node is not responding.');
    }
    setIsCheckingNode(false);
  };

  const handleRefreshNodes = async () => {
    setIsRefreshingNodes(true);
    try {
      await apiFetch('/api/nodes/refresh', { method: 'POST' });
    } catch {
      // ignore error, still try reload list
    }
    await onReloadNodes(1);
    setIsRefreshingNodes(false);
  };

  const total = nodes.length;
  const pct = (count: number) => (total > 0 ? `${(count / total) * 100}%` : '0%');

  return (
    <div className="flex flex-col gap-6">
      {/* Add node */}
      <Card>
        <CardHeader>
          <CardTitle>Add node</CardTitle>
          <CardDescription>Connect a WireGuard node through its API.</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="flex flex-col gap-4" onSubmit={handleCreateNode}>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="flex flex-col gap-2">
                <Label htmlFor="address">Node address</Label>
                <Input
                  id="address"
                  name="address"
                  className="font-mono"
                  placeholder="https://10.0.0.1:51821"
                  value={nodeForm.address}
                  onChange={(event) =>
                    updateNodeForm((prev) => ({ ...prev, address: event.target.value }))
                  }
                  required
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="apiKey">API key (X_API_KEY)</Label>
                <Input
                  id="apiKey"
                  name="apiKey"
                  type="password"
                  autoComplete="off"
                  className="font-mono"
                  placeholder="••••••••••••"
                  value={nodeForm.apiKey}
                  onChange={(event) =>
                    updateNodeForm((prev) => ({ ...prev, apiKey: event.target.value }))
                  }
                  required
                />
              </div>
            </div>
            {nodeCheck?.ok ? (
              <Alert variant="success">
                <CheckCircle2 />
                <AlertDescription>
                  Node detected:{' '}
                  <span className="font-medium">{nodeCheck.serviceName ?? nodeCheck.address}</span>
                  {nodeCheck.version ? `, version ${nodeCheck.version}` : null}
                </AlertDescription>
              </Alert>
            ) : null}
            {nodeCheckError ? (
              <Alert variant="destructive">
                <AlertCircle />
                <AlertDescription>{nodeCheckError}</AlertDescription>
              </Alert>
            ) : null}
            <div className="flex flex-wrap justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={handleCheckNode}
                disabled={isCheckingNode}
              >
                {isCheckingNode ? 'Checking…' : 'Check node'}
              </Button>
              <Button type="submit" disabled={!nodeCheck?.ok || isCreatingNode}>
                {isCreatingNode ? 'Saving…' : 'Save node'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {/* Nodes */}
      <Card>
        <CardHeader className="flex-row items-center justify-between gap-4 space-y-0">
          <div className="flex flex-col gap-1">
            <CardTitle>Nodes</CardTitle>
            <CardDescription>
              {latestRelease ? (
                <>
                  Latest wgkeeper-node v{latestRelease.version}
                  {latestRelease.url ? (
                    <>
                      {' · '}
                      <a
                        href={latestRelease.url}
                        target="_blank"
                        rel="noreferrer"
                        className="font-medium text-foreground underline-offset-4 hover:underline"
                      >
                        release notes
                      </a>
                    </>
                  ) : null}
                </>
              ) : (
                'Connected WireGuard nodes'
              )}
            </CardDescription>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant="secondary" className="tabular-nums">
              {nodesTotal}
            </Badge>
            <Button
              variant="outline"
              size="icon"
              className="size-8"
              disabled={isRefreshingNodes}
              title="Refresh"
              aria-label="Refresh nodes"
              onClick={handleRefreshNodes}
            >
              <RefreshCw className={`size-4 ${isRefreshingNodes ? 'animate-spin' : ''}`} />
            </Button>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {nodesError ? (
            <div className="px-6 pb-6">
              <Alert variant="destructive">
                <AlertCircle />
                <AlertDescription>{nodesError}</AlertDescription>
              </Alert>
            </div>
          ) : nodesLoading ? (
            <div className="divide-y divide-border border-t border-border">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="flex items-center gap-4 px-6 py-[0.9375rem]">
                  <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                    <Skeleton className="h-4 w-48" />
                    <Skeleton className="h-3 w-64" />
                  </div>
                  <Skeleton className="h-5 w-16 rounded-md" />
                  <Skeleton className="size-4 rounded-sm" />
                </div>
              ))}
            </div>
          ) : nodes.length ? (
            <>
              {/* Status summary */}
              <div className="flex flex-col gap-2 border-t border-border px-6 py-4">
                <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                  <span>
                    Online{' '}
                    <span className="font-medium text-success-foreground tabular-nums">
                      {statusSummary.online}
                    </span>
                  </span>
                  <span>
                    Offline{' '}
                    <span className="font-medium text-destructive tabular-nums">
                      {statusSummary.offline}
                    </span>
                  </span>
                  {statusSummary.other > 0 ? (
                    <span>
                      Other{' '}
                      <span className="font-medium text-warning-foreground tabular-nums">
                        {statusSummary.other}
                      </span>
                    </span>
                  ) : null}
                  {statusSummary.outdated > 0 ? (
                    <span>
                      Updates{' '}
                      <span className="font-medium text-warning-foreground tabular-nums">
                        {statusSummary.outdated}
                      </span>
                    </span>
                  ) : null}
                </div>
                <div className="flex h-1.5 w-full overflow-hidden rounded-full bg-muted">
                  {statusSummary.online > 0 ? (
                    <div
                      className="h-full bg-success"
                      style={{ width: pct(statusSummary.online) }}
                    />
                  ) : null}
                  {statusSummary.offline > 0 ? (
                    <div
                      className="h-full bg-destructive"
                      style={{ width: pct(statusSummary.offline) }}
                    />
                  ) : null}
                  {statusSummary.other > 0 ? (
                    <div
                      className="h-full bg-warning"
                      style={{ width: pct(statusSummary.other) }}
                    />
                  ) : null}
                </div>
              </div>

              {/* List */}
              <div className="divide-y divide-border border-t border-border">
                {nodes.map((node) => (
                  <Link
                    key={node.id}
                    to={`/nodes/${node.id}`}
                    className="group flex items-center gap-4 px-6 py-[0.9375rem] transition-colors hover:bg-accent/50"
                  >
                    <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                      <div className="flex items-center gap-2">
                        <span
                          className="truncate font-mono text-sm font-medium"
                          title={node.address}
                        >
                          {node.address}
                        </span>
                        {node.version ? (
                          <Badge
                            variant={node.isOutdated ? 'warning' : 'secondary'}
                            className="shrink-0 gap-1 font-mono text-[11px]"
                          >
                            {node.isOutdated ? <TriangleAlert /> : null}v{node.version}
                          </Badge>
                        ) : null}
                      </div>
                      <span
                        className="truncate font-mono text-xs text-muted-foreground"
                        title={node.id}
                      >
                        {node.id}
                      </span>
                    </div>
                    <NodeStatusBadge status={node.status} className="shrink-0" />
                    <ChevronRight className="size-4 shrink-0 text-muted-foreground/40 transition-colors group-hover:text-muted-foreground" />
                  </Link>
                ))}
              </div>

              {/* Pagination */}
              {nodesTotalPages > 1 ? (
                <div className="flex items-center justify-between gap-4 border-t border-border px-6 py-3">
                  <span className="text-xs text-muted-foreground tabular-nums">{pageLabel}</span>
                  <Pagination className="mx-0 w-auto justify-end">
                    <PaginationContent>
                      <PaginationItem>
                        <PaginationPrevious
                          href="#"
                          onClick={(e) => {
                            e.preventDefault();
                            void onReloadNodes(nodesPage - 1);
                          }}
                          aria-disabled={nodesPage === 1}
                          tabIndex={nodesPage === 1 ? -1 : undefined}
                          className={nodesPage === 1 ? 'pointer-events-none opacity-50' : ''}
                        />
                      </PaginationItem>
                      {Array.from({ length: nodesTotalPages }, (_, i) => i + 1)
                        .filter(
                          (page) =>
                            page === 1 ||
                            page === nodesTotalPages ||
                            Math.abs(page - nodesPage) <= 1,
                        )
                        .reduce<(number | 'ellipsis')[]>((acc, page, idx, arr) => {
                          if (idx > 0 && page - (arr[idx - 1] as number) > 1) acc.push('ellipsis');
                          acc.push(page);
                          return acc;
                        }, [])
                        .map((item, idx) =>
                          item === 'ellipsis' ? (
                            <PaginationItem key={`ellipsis-${idx}`}>
                              <PaginationEllipsis />
                            </PaginationItem>
                          ) : (
                            <PaginationItem key={item}>
                              <PaginationLink
                                href="#"
                                isActive={item === nodesPage}
                                onClick={(e) => {
                                  e.preventDefault();
                                  void onReloadNodes(item);
                                }}
                              >
                                {item}
                              </PaginationLink>
                            </PaginationItem>
                          ),
                        )}
                      <PaginationItem>
                        <PaginationNext
                          href="#"
                          onClick={(e) => {
                            e.preventDefault();
                            void onReloadNodes(nodesPage + 1);
                          }}
                          aria-disabled={nodesPage === nodesTotalPages}
                          tabIndex={nodesPage === nodesTotalPages ? -1 : undefined}
                          className={
                            nodesPage === nodesTotalPages ? 'pointer-events-none opacity-50' : ''
                          }
                        />
                      </PaginationItem>
                    </PaginationContent>
                  </Pagination>
                </div>
              ) : null}
            </>
          ) : (
            <div className="flex flex-col items-center gap-3 border-t border-border px-6 py-14 text-center">
              <div className="flex size-11 items-center justify-center rounded-lg border border-border bg-muted/50 text-muted-foreground">
                <Server className="size-5" />
              </div>
              <div className="flex flex-col gap-1">
                <p className="text-sm font-medium text-foreground">No nodes yet</p>
                <p className="max-w-xs text-sm text-muted-foreground">
                  Add a WireGuard node above to start managing peers and issuing configs.
                </p>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
};
