import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { AlertCircle, CheckCircle2, ExternalLink, RefreshCw, TriangleAlert } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
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
      try {
        const body = (await res.json()) as { error?: string };
        if (body?.error === 'node_exists' || res.status === 409) {
          setNodeCheckError('A node with this address already exists.');
        }
      } catch {
        if (res.status === 409) {
          setNodeCheckError('A node with this address already exists.');
        }
      }
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
      let errorMessage = 'Node is not responding.';
      try {
        const errorBody = (await res.json()) as { error?: string };
        if (errorBody?.error === 'invalid_api_key' || res.status === 401 || res.status === 403) {
          errorMessage = 'Invalid X_API_KEY.';
        }
      } catch {
        if (res.status === 401 || res.status === 403) {
          errorMessage = 'Invalid X_API_KEY.';
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

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader className="flex flex-col gap-1">
          <CardTitle>Add node</CardTitle>
          <CardDescription>Connect a WireGuard node via its API.</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="grid gap-4 md:grid-cols-2" onSubmit={handleCreateNode}>
            <div className="flex flex-col gap-1.5">
              <Label
                htmlFor="address"
                className="text-xs font-medium tracking-wide text-muted-foreground uppercase"
              >
                Node address
              </Label>
              <Input
                id="address"
                name="address"
                placeholder="https://10.0.0.1:51821"
                value={nodeForm.address}
                onChange={(event) =>
                  updateNodeForm((prev) => ({ ...prev, address: event.target.value }))
                }
                required
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label
                htmlFor="apiKey"
                className="text-xs font-medium tracking-wide text-muted-foreground uppercase"
              >
                X_API_KEY
              </Label>
              <Input
                id="apiKey"
                name="apiKey"
                type="password"
                autoComplete="off"
                placeholder="key-123"
                value={nodeForm.apiKey}
                onChange={(event) =>
                  updateNodeForm((prev) => ({ ...prev, apiKey: event.target.value }))
                }
                required
              />
            </div>
            {nodeCheck?.ok ? (
              <Alert className="border-emerald-200 bg-emerald-50 text-emerald-800 md:col-span-2 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">
                <CheckCircle2 className="size-4 !text-emerald-600 dark:!text-emerald-400" />
                <AlertDescription>
                  Node detected:{' '}
                  <span className="font-medium">{nodeCheck.serviceName ?? nodeCheck.address}</span>
                  {nodeCheck.version ? `, version ${nodeCheck.version}` : null}
                </AlertDescription>
              </Alert>
            ) : null}
            {nodeCheckError ? (
              <Alert variant="destructive" className="md:col-span-2">
                <AlertCircle className="size-4" />
                <AlertDescription>{nodeCheckError}</AlertDescription>
              </Alert>
            ) : null}
            <div className="flex flex-wrap justify-end gap-2 md:col-span-2">
              <Button
                type="button"
                variant="outline"
                onClick={handleCheckNode}
                disabled={isCheckingNode}
              >
                {isCheckingNode ? 'Checking...' : 'Check node'}
              </Button>
              <Button type="submit" disabled={!nodeCheck?.ok || isCreatingNode}>
                {isCreatingNode ? 'Saving...' : 'Save node'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex-row items-center justify-between space-y-0">
          <div>
            <CardTitle>Nodes list</CardTitle>
            <CardDescription>
              {latestRelease ? (
                <>
                  Latest wgkeeper-node: v{latestRelease.version}
                  {latestRelease.url ? (
                    <>
                      {' '}
                      <a
                        href={latestRelease.url}
                        target="_blank"
                        rel="noreferrer"
                        className="underline underline-offset-4"
                      >
                        release notes
                      </a>
                    </>
                  ) : null}
                </>
              ) : (
                'Last updated today'
              )}
            </CardDescription>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant="secondary">{nodesTotal} total</Badge>
            <Button
              variant="outline"
              size="icon"
              disabled={isRefreshingNodes}
              title="Refresh"
              onClick={async () => {
                setIsRefreshingNodes(true);
                try {
                  await apiFetch('/api/nodes/refresh', { method: 'POST' });
                } catch {
                  // ignore error, still try reload list
                }
                await onReloadNodes(1);
                setIsRefreshingNodes(false);
              }}
            >
              <RefreshCw className={`size-4 ${isRefreshingNodes ? 'animate-spin' : ''}`} />
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {nodesError ? (
            <Alert variant="destructive">
              <AlertCircle className="size-4" />
              <AlertDescription>{nodesError}</AlertDescription>
            </Alert>
          ) : nodesLoading ? (
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <Card key={i} className="border-border">
                  <CardHeader className="pb-3">
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2">
                        <Skeleton className="size-2.5 rounded-full" />
                        <Skeleton className="h-4 w-32" />
                      </div>
                      <Skeleton className="h-3 w-20" />
                    </div>
                  </CardHeader>
                  <CardContent className="flex items-end justify-between gap-4 pt-0">
                    <div className="flex flex-col gap-1.5">
                      <Skeleton className="h-2.5 w-8" />
                      <Skeleton className="h-4 w-56" />
                    </div>
                    <Skeleton className="h-8 w-16 rounded-md" />
                  </CardContent>
                </Card>
              ))}
            </div>
          ) : nodes.length ? (
            <div className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
                  <span>
                    Online:{' '}
                    <span className="font-medium text-emerald-600">{statusSummary.online}</span>
                  </span>
                  <span>
                    Offline:{' '}
                    <span className="font-medium text-red-600">{statusSummary.offline}</span>
                  </span>
                  {statusSummary.other > 0 ? (
                    <span>
                      Other:{' '}
                      <span className="font-medium text-amber-600">{statusSummary.other}</span>
                    </span>
                  ) : null}
                  {statusSummary.outdated > 0 ? (
                    <span>
                      Updates:{' '}
                      <span className="font-medium text-amber-600">{statusSummary.outdated}</span>
                    </span>
                  ) : null}
                </div>
                <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                  <div className="flex h-full w-full">
                    {statusSummary.online > 0 ? (
                      <div
                        className="h-full bg-emerald-500"
                        style={{ width: `${(statusSummary.online / nodes.length) * 100}%` }}
                      />
                    ) : null}
                    {statusSummary.offline > 0 ? (
                      <div
                        className="h-full bg-red-500"
                        style={{ width: `${(statusSummary.offline / nodes.length) * 100}%` }}
                      />
                    ) : null}
                    {statusSummary.other > 0 ? (
                      <div
                        className="h-full bg-amber-400"
                        style={{ width: `${(statusSummary.other / nodes.length) * 100}%` }}
                      />
                    ) : null}
                  </div>
                </div>
              </div>
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {nodes.map((node) => {
                  const statusColor =
                    node.status === 'online'
                      ? 'bg-emerald-500'
                      : node.status === 'offline'
                        ? 'bg-red-500'
                        : 'bg-amber-500';

                  const hostPortLabel = node.address;

                  return (
                    <Card key={node.id} className="border-border transition-shadow hover:shadow-md">
                      <CardContent className="flex items-center gap-3 px-4 py-4">
                        <span
                          className={`inline-block size-2 flex-shrink-0 rounded-full ${statusColor}`}
                        />
                        <div className="flex min-w-0 flex-1 flex-col gap-1">
                          <span
                            className="truncate font-mono text-sm font-medium"
                            title={hostPortLabel}
                          >
                            {hostPortLabel}
                          </span>
                          <div className="flex items-center gap-1.5">
                            {node.version ? (
                              <Badge
                                variant={node.isOutdated ? 'outline' : 'secondary'}
                                className={`flex-shrink-0 font-mono text-[10px] ${
                                  node.isOutdated
                                    ? 'gap-1.5 border-foreground/50 bg-foreground/[0.06] text-foreground shadow-[inset_0_0_0_1px_hsl(var(--foreground)/0.08)] dark:bg-foreground/[0.08]'
                                    : ''
                                }`}
                              >
                                {node.isOutdated ? (
                                  <TriangleAlert className="size-3 text-amber-500" />
                                ) : null}
                                v{node.version}
                              </Badge>
                            ) : null}
                            <span
                              className="truncate font-mono text-[11px] text-muted-foreground"
                              title={node.id}
                            >
                              {node.id}
                            </span>
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          <Button
                            variant="outline"
                            size="icon"
                            asChild
                            className="size-8 flex-shrink-0"
                          >
                            <Link to={`/nodes/${node.id}`} title="Open">
                              <ExternalLink className="size-3.5" />
                            </Link>
                          </Button>
                        </div>
                      </CardContent>
                    </Card>
                  );
                })}
              </div>
              {nodesTotalPages > 1 ? (
                <div className="flex items-center justify-between">
                  <span className="text-xs text-muted-foreground">{pageLabel}</span>
                  <Pagination className="flex-1 justify-end">
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
            </div>
          ) : (
            <div className="rounded-lg border border-dashed border-border px-6 py-10 text-center text-sm text-muted-foreground">
              Nodes have not been added yet.
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
};
