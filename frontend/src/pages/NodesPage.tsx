import { useDeferredValue, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  type ColumnDef,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  useReactTable,
} from '@tanstack/react-table';
import {
  ArrowUpCircleIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CircleAlertIcon,
  CopyIcon,
  ExternalLinkIcon,
  MoreHorizontalIcon,
  PlusIcon,
  RefreshCwIcon,
  SearchIcon,
  ServerIcon,
  Trash2Icon,
} from 'lucide-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
import { InputGroup, InputGroupAddon, InputGroupInput } from '@/components/ui/input-group';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { copyToClipboard } from '@/components/CopyButton';
import { PageHeader } from '@/components/layout/PageHeader';
import { AddNodeDialog } from '@/components/nodes/AddNodeDialog';
import { DeleteNodeDialog } from '@/components/nodes/DeleteNodeDialog';
import { NodeStatusBadge } from '@/components/nodes/NodeStatusBadge';
import { nodeHost, nodeScheme, shortId } from '@/lib/node-utils';
import { NODES_PAGE_SIZE, formatDate, formatRelative } from '@/lib/peer-utils';
import { cn } from '@/lib/utils';
import { usePatchSearchParams, useSearchParam } from '@/hooks/use-search-param';
import type { FleetSummary, NodeItem } from '@/types';

type Props = {
  nodes: NodeItem[];
  nodesPage: number;
  nodesTotalPages: number;
  nodesTotal: number;
  nodesLoading?: boolean;
  nodesError?: string | null;
  nodesSummary: FleetSummary | null;
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  onReloadNodes: (page?: number) => Promise<boolean>;
  onRefreshNodes: () => Promise<void>;
};

type StatusFilter = 'all' | 'online' | 'offline' | 'outdated';

const STATUS_FILTERS = new Set<string>(['all', 'online', 'offline', 'outdated']);

// Stable keys for loading placeholders: there's no data to key them by yet.
const SKELETON_ROWS = ['row-1', 'row-2', 'row-3', 'row-4', 'row-5'];

const matchesStatus = (node: NodeItem, filter: StatusFilter) => {
  if (filter === 'all') return true;
  if (filter === 'outdated') return node.isOutdated;
  return node.status === filter;
};

const columns: ColumnDef<NodeItem>[] = [
  {
    id: 'node',
    header: 'Node',
    accessorFn: (node) => `${node.address} ${node.id} ${node.version ?? ''}`,
    cell: ({ row }) => (
      <div className="flex min-w-0 flex-col gap-0.5">
        <Link
          to={`/nodes/${row.original.id}`}
          className="truncate rounded-sm font-mono text-[0.8125rem] font-medium outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
          onClick={(e) => e.stopPropagation()}
        >
          {nodeHost(row.original.address)}
        </Link>
        <span className="font-mono text-xs text-muted-foreground">{shortId(row.original.id)}</span>
      </div>
    ),
  },
  {
    id: 'status',
    header: 'Status',
    cell: ({ row }) => <NodeStatusBadge status={row.original.status} />,
  },
  {
    id: 'version',
    header: 'Version',
    cell: ({ row }) => <VersionCell node={row.original} />,
  },
  {
    id: 'api',
    header: 'API',
    cell: ({ row }) => (
      <span className="font-mono text-xs text-muted-foreground">
        {nodeScheme(row.original.address) ?? '—'}
      </span>
    ),
  },
  {
    id: 'added',
    header: 'Added',
    cell: ({ row }) => (
      <span className="text-muted-foreground" title={formatDate(row.original.createdAt)}>
        {formatRelative(row.original.createdAt)}
      </span>
    ),
  },
  {
    id: 'actions',
    header: () => <span className="sr-only">Actions</span>,
    cell: ({ row, table }) => (
      <NodeActions
        node={row.original}
        onOpen={() => table.options.meta?.onOpen?.(row.original.id)}
        onDelete={() => table.options.meta?.onDelete?.(row.original.id)}
      />
    ),
  },
];

export const NodesPage = ({
  nodes,
  nodesPage,
  nodesTotalPages,
  nodesTotal,
  nodesLoading,
  nodesError,
  nodesSummary,
  apiFetch,
  onReloadNodes,
  onRefreshNodes,
}: Props) => {
  const navigate = useNavigate();
  // Filters, the page and the open dialog live in the URL, so reload and the
  // back button keep them.
  const [addParam, setAddParam] = useSearchParam('add');
  const [search, setSearch] = useSearchParam('q');
  const [statusParam, setStatusParam] = useSearchParam('status', 'all');
  const [pageParam] = useSearchParam('page', '1');
  const patchParams = usePatchSearchParams();
  const statusFilter: StatusFilter = STATUS_FILTERS.has(statusParam)
    ? (statusParam as StatusFilter)
    : 'all';
  const requestedPage = Math.max(1, Math.trunc(Number(pageParam)) || 1);
  const deferredSearch = useDeferredValue(search);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [deleteNode, setDeleteNode] = useState<NodeItem | null>(null);

  const addOpen = addParam === '1';
  const setAddOpen = (open: boolean) => setAddParam(open ? '1' : '');
  const goToPage = (page: number) => patchParams({ page: page > 1 ? String(page) : null });

  // Follow the page in the URL; App keeps the list it loaded last.
  useEffect(() => {
    if (requestedPage !== nodesPage) void onReloadNodes(requestedPage);
    // Only the URL drives this: nodesPage catching up must not trigger a reload.
    // oxlint-disable-next-line react/exhaustive-deps
  }, [requestedPage]);

  const latest = useMemo(() => nodes.find((n) => n.latestVersion) ?? null, [nodes]);

  const filtered = useMemo(
    () => nodes.filter((node) => matchesStatus(node, statusFilter)),
    [nodes, statusFilter],
  );

  const table = useReactTable({
    data: filtered,
    columns,
    state: { globalFilter: deferredSearch },
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getRowId: (node) => node.id,
    meta: {
      onOpen: (id) => void navigate(`/nodes/${id}`),
      onDelete: (id) => setDeleteNode(nodes.find((node) => node.id === id) ?? null),
    },
  });

  const handleRefresh = async () => {
    setIsRefreshing(true);
    await onRefreshNodes();
    setIsRefreshing(false);
  };

  const from = (nodesPage - 1) * NODES_PAGE_SIZE + 1;
  const to = Math.min(nodesPage * NODES_PAGE_SIZE, nodesTotal);
  const rows = table.getRowModel().rows;
  const hasNodes = nodes.length > 0;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Nodes"
        description="WireGuard nodes this console manages through their API."
        actions={
          <>
            <Button variant="outline" disabled={isRefreshing} onClick={() => void handleRefresh()}>
              <RefreshCwIcon className={cn(isRefreshing && 'animate-spin')} />
              Refresh
            </Button>
            <Button onClick={() => setAddOpen(true)}>
              <PlusIcon />
              Add node
            </Button>
          </>
        }
      />

      {hasNodes ? (
        <div className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border bg-border shadow-xs lg:grid-cols-4">
          <Summary label="Nodes" value={nodesTotal} />
          <Summary label="Online" value={nodesSummary?.online} tone="success" />
          <Summary
            label="Offline"
            value={nodesSummary?.offline}
            tone={(nodesSummary?.offline ?? 0) > 0 ? 'destructive' : undefined}
          />
          <Summary
            label="Updates available"
            value={nodesSummary?.outdated}
            tone={(nodesSummary?.outdated ?? 0) > 0 ? 'warning' : undefined}
            hint={
              latest?.latestVersion ? (
                latest.latestVersionUrl ? (
                  <a
                    href={latest.latestVersionUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="underline-offset-4 hover:text-foreground hover:underline"
                  >
                    Latest v{latest.latestVersion}
                  </a>
                ) : (
                  `Latest v${latest.latestVersion}`
                )
              ) : undefined
            }
          />
        </div>
      ) : null}

      {nodesError ? (
        <Alert variant="destructive">
          <CircleAlertIcon />
          <AlertTitle>Nodes could not be loaded</AlertTitle>
          <AlertDescription>
            <p>{nodesError}</p>
            <Button
              variant="outline"
              size="sm"
              className="mt-2"
              onClick={() => void onReloadNodes()}
            >
              Try again
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      {!nodesLoading && !nodesError && !hasNodes ? (
        <Empty className="border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <ServerIcon />
            </EmptyMedia>
            <EmptyTitle>No nodes yet</EmptyTitle>
            <EmptyDescription>
              Add a wgkeeper node to issue client configs and manage its peers from here.
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button onClick={() => setAddOpen(true)}>
              <PlusIcon />
              Add node
            </Button>
          </EmptyContent>
        </Empty>
      ) : null}

      {nodesLoading || hasNodes ? (
        <div className="flex flex-col gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <InputGroup className="w-full sm:w-72">
              <InputGroupInput
                placeholder="Filter by address, ID or version"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                aria-label="Filter nodes"
              />
              <InputGroupAddon>
                <SearchIcon />
              </InputGroupAddon>
            </InputGroup>
            <ToggleGroup
              type="single"
              variant="outline"
              size="sm"
              value={statusFilter}
              onValueChange={(value) => value && setStatusParam(value)}
              aria-label="Filter by status"
            >
              <ToggleGroupItem value="all" className="px-3">
                All
              </ToggleGroupItem>
              <ToggleGroupItem value="online" className="px-3">
                Online
              </ToggleGroupItem>
              <ToggleGroupItem value="offline" className="px-3">
                Offline
              </ToggleGroupItem>
              <ToggleGroupItem value="outdated" className="px-3">
                Outdated
              </ToggleGroupItem>
            </ToggleGroup>
          </div>

          <div className="overflow-hidden rounded-lg border">
            <Table>
              <TableHeader className="bg-muted/50">
                {table.getHeaderGroups().map((group) => (
                  <TableRow key={group.id} className="hover:bg-transparent">
                    {group.headers.map((header) => (
                      <TableHead
                        key={header.id}
                        className={cn(
                          'h-9 text-xs',
                          header.id === 'actions' && 'w-12',
                          (header.id === 'api' || header.id === 'added') && 'max-md:hidden',
                        )}
                      >
                        {flexRender(header.column.columnDef.header, header.getContext())}
                      </TableHead>
                    ))}
                  </TableRow>
                ))}
              </TableHeader>
              <TableBody>
                {nodesLoading && !hasNodes ? (
                  SKELETON_ROWS.map((key) => (
                    <TableRow key={key}>
                      <TableCell>
                        <Skeleton className="h-4 w-48" />
                        <Skeleton className="mt-1.5 h-3 w-16" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-5 w-16" />
                      </TableCell>
                      <TableCell>
                        <Skeleton className="h-5 w-14" />
                      </TableCell>
                      <TableCell className="max-md:hidden">
                        <Skeleton className="h-4 w-10" />
                      </TableCell>
                      <TableCell className="max-md:hidden">
                        <Skeleton className="h-4 w-20" />
                      </TableCell>
                      <TableCell />
                    </TableRow>
                  ))
                ) : rows.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell colSpan={columns.length} className="h-24 text-center">
                      <span className="text-muted-foreground">
                        No nodes on this page match the filter.
                      </span>{' '}
                      <Button
                        variant="link"
                        className="h-auto p-0"
                        onClick={() => {
                          patchParams({ q: null, status: null });
                        }}
                      >
                        Clear filters
                      </Button>
                    </TableCell>
                  </TableRow>
                ) : (
                  rows.map((row) => (
                    <TableRow
                      key={row.id}
                      className="cursor-pointer"
                      onClick={() => void navigate(`/nodes/${row.original.id}`)}
                    >
                      {row.getVisibleCells().map((cell) => (
                        <TableCell
                          key={cell.id}
                          className={cn(
                            'py-2.5',
                            (cell.column.id === 'api' || cell.column.id === 'added') &&
                              'max-md:hidden',
                          )}
                        >
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>

          {nodesTotalPages > 1 ? (
            <div className="flex items-center justify-between gap-4 text-sm">
              <span className="text-muted-foreground tabular-nums">
                {from}–{to} of {nodesTotal}
              </span>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="icon-sm"
                  aria-label="Previous page"
                  disabled={nodesPage <= 1}
                  onClick={() => goToPage(nodesPage - 1)}
                >
                  <ChevronLeftIcon />
                </Button>
                <span className="min-w-16 text-center tabular-nums">
                  {nodesPage} / {nodesTotalPages}
                </span>
                <Button
                  variant="outline"
                  size="icon-sm"
                  aria-label="Next page"
                  disabled={nodesPage >= nodesTotalPages}
                  onClick={() => goToPage(nodesPage + 1)}
                >
                  <ChevronRightIcon />
                </Button>
              </div>
            </div>
          ) : null}
        </div>
      ) : null}

      <AddNodeDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        apiFetch={apiFetch}
        onAdded={async () => {
          // New nodes come first; show the page they land on.
          if (nodesPage === 1) await onReloadNodes(1);
          else goToPage(1);
        }}
      />
      <DeleteNodeDialog
        node={deleteNode}
        onOpenChange={(open) => {
          if (!open) setDeleteNode(null);
        }}
        apiFetch={apiFetch}
        onDeleted={() => void onReloadNodes()}
      />
    </div>
  );
};

const toneClass = {
  success: 'text-success-foreground',
  destructive: 'text-destructive',
  warning: 'text-warning-foreground',
} as const;

const Summary = ({
  label,
  value,
  tone,
  hint,
}: {
  label: string;
  value: number | undefined;
  tone?: keyof typeof toneClass;
  hint?: React.ReactNode;
}) => (
  <div className="flex flex-col gap-1 bg-card px-5 py-4">
    <span className="text-sm text-muted-foreground">{label}</span>
    <span
      className={cn(
        'text-2xl font-semibold tracking-tight tabular-nums',
        tone !== undefined && toneClass[tone],
      )}
    >
      {value ?? '—'}
    </span>
    {hint === undefined ? null : <span className="text-xs text-muted-foreground">{hint}</span>}
  </div>
);

const VersionCell = ({ node }: { node: NodeItem }) => {
  if (!node.version) return <span className="text-muted-foreground">—</span>;
  if (!node.isOutdated || !node.latestVersion) {
    return (
      <Badge variant="secondary" className="font-mono">
        v{node.version}
      </Badge>
    );
  }
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Badge
          variant="outline"
          className="gap-1 border-warning/40 font-mono text-warning-foreground"
        >
          <ArrowUpCircleIcon aria-hidden />v{node.version}
          <span className="sr-only">, v{node.latestVersion} is available</span>
        </Badge>
      </TooltipTrigger>
      <TooltipContent>v{node.latestVersion} is available</TooltipContent>
    </Tooltip>
  );
};

const NodeActions = ({
  node,
  onOpen,
  onDelete,
}: {
  node: NodeItem;
  onOpen: () => void;
  onDelete: () => void;
}) => (
  <DropdownMenu>
    <DropdownMenuTrigger asChild>
      <Button
        variant="ghost"
        size="icon-sm"
        className="text-muted-foreground data-[state=open]:bg-accent"
        aria-label={`Actions for ${nodeHost(node.address)}`}
        onClick={(e) => e.stopPropagation()}
      >
        <MoreHorizontalIcon />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" className="w-48" onClick={(e) => e.stopPropagation()}>
      <DropdownMenuItem onSelect={onOpen}>
        <ExternalLinkIcon />
        Open
      </DropdownMenuItem>
      <DropdownMenuItem onSelect={() => void copyToClipboard(node.address, 'Address copied')}>
        <CopyIcon />
        Copy address
      </DropdownMenuItem>
      <DropdownMenuItem onSelect={() => void copyToClipboard(node.id, 'Node ID copied')}>
        <CopyIcon />
        Copy node ID
      </DropdownMenuItem>
      <DropdownMenuSeparator />
      <DropdownMenuItem variant="destructive" onSelect={onDelete}>
        <Trash2Icon />
        Remove node
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
);
