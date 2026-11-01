import { Fragment, Suspense, lazy, useCallback, useEffect, useState } from 'react';
import { Link, Navigate, Route, Routes, useLocation, useParams } from 'react-router-dom';
import { SearchIcon, ServerOffIcon } from 'lucide-react';
import { toast } from 'sonner';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb';
import { Button } from '@/components/ui/button';
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import { Kbd } from '@/components/ui/kbd';
import { Separator } from '@/components/ui/separator';
import { SidebarInset, SidebarProvider, SidebarTrigger } from '@/components/ui/sidebar';
import { Skeleton } from '@/components/ui/skeleton';
import { AppSidebar } from '@/components/layout/AppSidebar';
import { CommandMenu } from '@/components/layout/CommandMenu';
import { nodeHost, shortId } from '@/lib/node-utils';
import type { FleetSummary, NodeItem, NodeListResponse } from '@/types';

const loadNodeDetailPage = () => import('@/pages/NodeDetailPage');
const NodesPage = lazy(() => import('@/pages/NodesPage').then((m) => ({ default: m.NodesPage })));
const NodeDetailPage = lazy(() =>
  loadNodeDetailPage().then((m) => ({ default: m.NodeDetailPage })),
);
const ProfilePage = lazy(() =>
  import('@/pages/ProfilePage').then((m) => ({ default: m.ProfilePage })),
);

// The sidebar and the command menu list the whole fleet, not the current page.
const FLEET_LIMIT = 100;

const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/u.test(navigator.userAgent);

type ApiFetch = (path: string, options?: RequestInit) => Promise<Response>;

type AppShellProps = {
  nodes: NodeItem[];
  nodesPage: number;
  nodesTotalPages: number;
  nodesTotal: number;
  nodesLoading?: boolean;
  nodesError?: string | null;
  nodesSummary: FleetSummary | null;
  apiFetch: ApiFetch;
  onReloadNodes: (page?: number) => Promise<boolean>;
  onLogout: () => void;
  username?: string;
};

type Crumb = { label: string; to?: string; mono?: boolean };

export const AppShell = ({
  nodes,
  nodesPage,
  nodesTotalPages,
  nodesTotal,
  nodesLoading,
  nodesError,
  nodesSummary,
  apiFetch,
  onReloadNodes,
  onLogout,
  username,
}: AppShellProps) => {
  const { pathname } = useLocation();
  const [commandOpen, setCommandOpen] = useState(false);
  const [fleet, setFleet] = useState<NodeItem[]>(nodes);

  // Refetch the fleet whenever the current page reloads, so both stay in step.
  useEffect(() => {
    const controller = new AbortController();
    const load = async () => {
      try {
        const res = await apiFetch(`/api/nodes?page=1&limit=${FLEET_LIMIT}`, {
          signal: controller.signal,
        });
        if (!res.ok) return;
        setFleet(((await res.json()) as NodeListResponse).nodes);
      } catch {
        // keep the last known fleet
      }
    };
    void load();
    return () => controller.abort();
    // nodes isn't read inside: a reloaded page is the cue to refetch the fleet.
    // oxlint-disable-next-line react/exhaustive-effect-dependencies
  }, [apiFetch, nodes]);

  // Most sessions open a node next; fetch its page's code while idle.
  useEffect(() => {
    void loadNodeDetailPage();
  }, []);

  const detailMatch = /^\/nodes\/([^/]+)/u.exec(pathname);
  const detailId = detailMatch?.[1];
  const detailNode = detailId ? fleet.find((n) => n.id === detailId) : undefined;

  let crumbs: Crumb[];
  if (pathname.startsWith('/profile')) crumbs = [{ label: 'Profile' }];
  else if (detailId)
    crumbs = [
      { label: 'Nodes', to: '/nodes' },
      { label: detailNode ? nodeHost(detailNode.address) : shortId(detailId), mono: true },
    ];
  else crumbs = [{ label: 'Nodes' }];

  const pageTitle = crumbs.at(-1)?.label ?? 'Nodes';
  useEffect(() => {
    document.title = `${pageTitle} · wgkeeper console`;
  }, [pageTitle]);

  const refreshNodes = useCallback(async () => {
    let refreshed = false;
    try {
      refreshed = (await apiFetch('/api/nodes/refresh', { method: 'POST' })).ok;
    } catch {
      // reported below
    }
    const reloaded = await onReloadNodes();
    if (refreshed && reloaded) toast.success('Node status refreshed');
    else toast.error('Node status could not be refreshed. Showing the last known state.');
  }, [apiFetch, onReloadNodes]);

  return (
    <SidebarProvider>
      <a
        href="#main"
        className="sr-only z-50 rounded-md bg-background px-3 py-2 text-sm font-medium focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:ring-2 focus:ring-ring"
      >
        Skip to content
      </a>
      <AppSidebar nodes={fleet} nodesTotal={nodesTotal} username={username} onLogout={onLogout} />
      <SidebarInset>
        <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center gap-2 border-b bg-background/90 px-4 backdrop-blur md:rounded-t-xl">
          <SidebarTrigger className="-ml-1 pointer-coarse:size-9" />
          <Separator orientation="vertical" className="mr-1 data-[orientation=vertical]:h-4" />
          <Breadcrumb className="min-w-0">
            <BreadcrumbList className="flex-nowrap">
              {crumbs.map((crumb, index) => (
                <Fragment key={crumb.label}>
                  {index > 0 ? <BreadcrumbSeparator /> : null}
                  <BreadcrumbItem className="min-w-0">
                    {crumb.to === undefined ? (
                      <BreadcrumbPage
                        className={
                          crumb.mono === true ? 'truncate font-mono text-[0.8125rem]' : 'truncate'
                        }
                      >
                        {crumb.label}
                      </BreadcrumbPage>
                    ) : (
                      <BreadcrumbLink asChild>
                        <Link to={crumb.to}>{crumb.label}</Link>
                      </BreadcrumbLink>
                    )}
                  </BreadcrumbItem>
                </Fragment>
              ))}
            </BreadcrumbList>
          </Breadcrumb>
          <Button
            variant="outline"
            size="sm"
            className="ml-auto h-8 w-9 justify-start gap-2 px-2.5 text-muted-foreground sm:w-56 pointer-coarse:size-9"
            aria-label="Search nodes and actions"
            aria-keyshortcuts={isMac ? 'Meta+K' : 'Control+K'}
            onClick={() => setCommandOpen(true)}
          >
            <SearchIcon />
            <span className="hidden sm:inline">Search…</span>
            <Kbd className="ml-auto hidden sm:inline-flex">{isMac ? '⌘K' : 'Ctrl K'}</Kbd>
          </Button>
        </header>

        <main
          id="main"
          tabIndex={-1}
          className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 outline-none sm:px-6 lg:py-8"
        >
          <Suspense fallback={<PageSkeleton />}>
            <Routes>
              <Route path="/" element={<Navigate to="/nodes" replace />} />
              <Route path="/login" element={<Navigate to="/nodes" replace />} />
              <Route
                path="/nodes"
                element={
                  <NodesPage
                    nodes={nodes}
                    nodesPage={nodesPage}
                    nodesTotalPages={nodesTotalPages}
                    nodesTotal={nodesTotal}
                    nodesLoading={nodesLoading}
                    nodesError={nodesError}
                    nodesSummary={nodesSummary}
                    apiFetch={apiFetch}
                    onReloadNodes={onReloadNodes}
                    onRefreshNodes={refreshNodes}
                  />
                }
              />
              <Route
                path="/nodes/:id"
                element={
                  <NodeDetailRoute
                    fleet={fleet}
                    apiFetch={apiFetch}
                    onReloadNodes={onReloadNodes}
                  />
                }
              />
              <Route
                path="/profile"
                element={<ProfilePage apiFetch={apiFetch} initialUsername={username} />}
              />
              <Route path="*" element={<NotFound />} />
            </Routes>
          </Suspense>
        </main>
      </SidebarInset>
      <CommandMenu
        open={commandOpen}
        setOpen={setCommandOpen}
        nodes={fleet}
        onRefresh={() => void refreshNodes()}
        onLogout={onLogout}
      />
    </SidebarProvider>
  );
};

/**
 * Keys the detail page by node ID: moving from one node to another mounts a
 * fresh page instead of carrying over the previous node's stats, peers and
 * generated config.
 */
const NodeDetailRoute = ({
  fleet,
  apiFetch,
  onReloadNodes,
}: {
  fleet: NodeItem[];
  apiFetch: ApiFetch;
  onReloadNodes: (page?: number) => Promise<boolean>;
}) => {
  const { id = '' } = useParams<{ id: string }>();
  return (
    <NodeDetailPage
      key={id}
      nodeId={id}
      initialNode={fleet.find((n) => n.id === id) ?? null}
      apiFetch={apiFetch}
      onReloadNodes={onReloadNodes}
    />
  );
};

const PageSkeleton = () => (
  <div className="flex flex-col gap-6">
    <div className="flex flex-col gap-2">
      <Skeleton className="h-8 w-40" />
      <Skeleton className="h-4 w-72" />
    </div>
    <Skeleton className="h-24 w-full rounded-xl" />
    <Skeleton className="h-80 w-full rounded-xl" />
  </div>
);

const NotFound = () => (
  <Empty className="border border-dashed">
    <EmptyHeader>
      <EmptyMedia variant="icon">
        <ServerOffIcon />
      </EmptyMedia>
      <EmptyTitle>Page not found</EmptyTitle>
      <EmptyDescription>There is nothing at this address.</EmptyDescription>
    </EmptyHeader>
    <EmptyContent>
      <Button variant="outline" size="sm" asChild>
        <Link to="/nodes">Back to nodes</Link>
      </Button>
    </EmptyContent>
  </Empty>
);
