import { useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import {
  type ColumnDef,
  type SortingState,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table';
import {
  ArrowDownIcon,
  ArrowUpIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CircleAlertIcon,
  CopyIcon,
  FilePlus2Icon,
  MoreHorizontalIcon,
  PanelRightOpenIcon,
  RefreshCwIcon,
  SearchIcon,
  Trash2Icon,
  UsersIcon,
} from 'lucide-react';
import { toast } from 'sonner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Spinner } from '@/components/ui/spinner';
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
import { PeerDetailSheet } from '@/components/nodes/PeerDetailSheet';
import { PeerExpires } from '@/components/nodes/PeerExpires';
import { PeerStatusBadge } from '@/components/nodes/NodeStatusBadge';
import { usePatchSearchParams, useSearchParam } from '@/hooks/use-search-param';
import { fetchErrorMessage, rateLimitMessage } from '@/lib/api-client';
import { shortId } from '@/lib/node-utils';
import { PAGE_SIZES, formatDate, formatRelative, plural } from '@/lib/peer-utils';
import { cn } from '@/lib/utils';
import type { PeerListItem, PeersResponse } from '@/types';

// Stable keys for loading placeholders: there's no data to key them by yet.
const SKELETON_ROWS = ['row-1', 'row-2', 'row-3', 'row-4', 'row-5', 'row-6'];
const DEFAULT_LIMIT = '50';
const ACTIVITY_FILTERS = new Set(['all', 'active', 'inactive']);

type ActivityFilter = 'all' | 'active' | 'inactive';

type Props = {
  nodeId: string;
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  /** When this value changes, peers list is refetched (e.g. after config generated) */
  refreshTrigger?: number;
  onPeersChanged?: () => void;
  onNewConfig?: () => void;
};

// Columns that give way first on narrow screens.
const responsiveClass = (id: string) =>
  id === 'key' || id === 'created'
    ? 'max-lg:hidden'
    : id === 'handshake' || id === 'expires'
      ? 'max-md:hidden'
      : undefined;

const columns: ColumnDef<PeerListItem>[] = [
  {
    id: 'peer',
    header: 'Peer',
    accessorFn: (peer) => `${peer.peerId} ${peer.allowedIPs?.join(' ')} ${peer.publicKey}`,
    // The ID is the row's keyboard entry point: the row click is for the mouse.
    cell: ({ row, table }) => (
      <button
        type="button"
        className="rounded-sm font-mono text-[0.8125rem] font-medium outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
        title={row.original.peerId}
        aria-label={`Open peer ${shortId(row.original.peerId)}`}
        onClick={(e) => {
          e.stopPropagation();
          table.options.meta?.onOpen?.(row.original.peerId);
        }}
      >
        {shortId(row.original.peerId)}
      </button>
    ),
  },
  {
    id: 'ips',
    header: 'Allowed IPs',
    cell: ({ row }) => (
      <span className="font-mono text-[0.8125rem]">
        {row.original.allowedIPs?.join(', ') ?? '—'}
      </span>
    ),
  },
  {
    id: 'key',
    header: 'Public key',
    cell: ({ row }) => (
      <span
        className="block max-w-36 truncate font-mono text-xs text-muted-foreground"
        title={row.original.publicKey}
      >
        {row.original.publicKey}
      </span>
    ),
  },
  {
    id: 'status',
    header: 'Status',
    cell: ({ row }) => <PeerStatusBadge active={row.original.active} />,
  },
  {
    id: 'handshake',
    header: 'Last handshake',
    cell: ({ row }) =>
      row.original.lastHandshakeAt ? (
        <time
          dateTime={row.original.lastHandshakeAt}
          className="text-muted-foreground"
          title={formatDate(row.original.lastHandshakeAt)}
        >
          {formatRelative(row.original.lastHandshakeAt)}
        </time>
      ) : (
        <span className="text-muted-foreground">Never</span>
      ),
  },
  {
    id: 'created',
    accessorFn: (peer) => (peer.createdAt ? new Date(peer.createdAt).getTime() : 0),
    header: ({ column }) => (
      <button
        type="button"
        className="-ml-1 inline-flex items-center gap-1 rounded px-1 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}
      >
        Created
        {column.getIsSorted() === 'asc' ? (
          <ArrowUpIcon className="size-3.5" />
        ) : (
          <ArrowDownIcon className="size-3.5" />
        )}
      </button>
    ),
    cell: ({ row }) => (
      <time
        dateTime={row.original.createdAt}
        className="text-muted-foreground"
        title={formatDate(row.original.createdAt)}
      >
        {formatRelative(row.original.createdAt)}
      </time>
    ),
  },
  {
    id: 'expires',
    header: 'Expires',
    cell: ({ row, table }) => (
      <PeerExpires iso={row.original.expiresAt} now={table.options.meta?.now ?? 0} />
    ),
  },
  {
    id: 'actions',
    header: () => <span className="sr-only">Actions</span>,
    cell: ({ row, table }) => (
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            className="text-muted-foreground data-[state=open]:bg-accent"
            aria-label={`Actions for peer ${shortId(row.original.peerId)}`}
            onClick={(e) => e.stopPropagation()}
          >
            <MoreHorizontalIcon />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-48" onClick={(e) => e.stopPropagation()}>
          <DropdownMenuItem onSelect={() => table.options.meta?.onOpen?.(row.original.peerId)}>
            <PanelRightOpenIcon />
            Details
          </DropdownMenuItem>
          <DropdownMenuItem
            onSelect={() => void copyToClipboard(row.original.peerId, 'Peer ID copied')}
          >
            <CopyIcon />
            Copy peer ID
          </DropdownMenuItem>
          <DropdownMenuItem
            onSelect={() => void copyToClipboard(row.original.publicKey, 'Public key copied')}
          >
            <CopyIcon />
            Copy public key
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            variant="destructive"
            onSelect={() => table.options.meta?.onDelete?.(row.original.peerId)}
          >
            <Trash2Icon />
            Delete peer
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    ),
  },
];

export const NodePeersTab = ({
  nodeId,
  apiFetch,
  refreshTrigger = 0,
  onPeersChanged,
  onNewConfig,
}: Props) => {
  // Page, filters and the open peer live in the URL, so reload, back and a
  // shared link keep them.
  const [offsetParam] = useSearchParam('poffset', '0');
  const [limitParam] = useSearchParam('plimit', DEFAULT_LIMIT);
  const [search, setSearch] = useSearchParam('pq');
  const [activityParam, setActivity] = useSearchParam('pfilter', 'all');
  const [detailPeerId, setDetailPeerIdParam] = useSearchParam('peer');
  const patchParams = usePatchSearchParams();
  const offset = Math.max(0, Math.trunc(Number(offsetParam)) || 0);
  const limit = PAGE_SIZES.find((size) => String(size) === limitParam) ?? Number(DEFAULT_LIMIT);
  const activity: ActivityFilter = ACTIVITY_FILTERS.has(activityParam)
    ? (activityParam as ActivityFilter)
    : 'all';
  const deferredSearch = useDeferredValue(search);

  const [peersData, setPeersData] = useState<PeersResponse | null>(null);
  const [peersLoading, setPeersLoading] = useState(false);
  const [peersError, setPeersError] = useState('');
  // When the list was fetched: "expires soon" is judged against this, not the render time.
  const [loadedAt, setLoadedAt] = useState(0);
  const [reloadKey, setReloadKey] = useState(0);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [sorting, setSorting] = useState<SortingState>([{ id: 'created', desc: true }]);

  const setDetailPeerId = (id: string | null) => setDetailPeerIdParam(id ?? '');
  const setOffset = (next: number) => patchParams({ poffset: next > 0 ? String(next) : null });

  // The one place peers are fetched. Every reason to reload (page, page size,
  // the parent's trigger, the reload button) is a dependency, so a newer
  // request always aborts the one before it and a late response can't land.
  useEffect(() => {
    const controller = new AbortController();
    setPeersLoading(true);
    setPeersError('');
    const load = async () => {
      try {
        const res = await apiFetch(
          `/api/nodes/${encodeURIComponent(nodeId)}/peers?offset=${offset}&limit=${limit}`,
          { signal: controller.signal },
        );
        if (!res.ok) {
          setPeersData(null);
          setPeersError('The node did not return its peers.');
          return;
        }
        setPeersData((await res.json()) as PeersResponse);
        setLoadedAt(Date.now());
      } catch (err) {
        if ((err as Error).name === 'AbortError') return;
        setPeersData(null);
        setPeersError('The node did not return its peers.');
      } finally {
        if (!controller.signal.aborted) setPeersLoading(false);
      }
    };
    void load();
    return () => controller.abort();
    // refreshTrigger and reloadKey aren't read inside: bumping them asks for a reload.
  }, [nodeId, offset, limit, apiFetch, refreshTrigger, reloadKey]);

  const deleteAbortRef = useRef<AbortController | null>(null);
  useEffect(() => () => deleteAbortRef.current?.abort(), []);

  const handleDeletePeer = async (peerId: string) => {
    deleteAbortRef.current?.abort();
    const controller = new AbortController();
    deleteAbortRef.current = controller;
    setIsDeleting(true);
    try {
      const res = await apiFetch(
        `/api/nodes/${encodeURIComponent(nodeId)}/peers?peerId=${encodeURIComponent(peerId)}`,
        { method: 'DELETE', signal: controller.signal },
      );
      if (res.ok) {
        if (detailPeerId === peerId) setDetailPeerId(null);
        setConfirmDeleteId(null);
        toast.success('Peer deleted', { description: shortId(peerId) });
        onPeersChanged?.();
        setReloadKey((k) => k + 1);
      } else {
        toast.error(rateLimitMessage(res) ?? 'Could not delete the peer.');
      }
    } catch (err) {
      if ((err as Error).name === 'AbortError') return;
      toast.error(fetchErrorMessage(err) ?? 'Could not delete the peer.');
    } finally {
      if (!controller.signal.aborted) setIsDeleting(false);
    }
  };

  const peers = useMemo(
    () =>
      (peersData?.data ?? []).filter(
        (peer) => activity === 'all' || peer.active === (activity === 'active'),
      ),
    [peersData, activity],
  );

  // TanStack Table returns fresh functions on every render, which is fine here:
  // this app does not run the React Compiler.
  // oxlint-disable-next-line react/incompatible-library
  const table = useReactTable({
    data: peers,
    columns,
    state: { globalFilter: deferredSearch, sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getRowId: (peer) => peer.peerId,
    meta: {
      now: loadedAt,
      onOpen: setDetailPeerId,
      onDelete: setConfirmDeleteId,
    },
  });

  const total = peersData?.meta?.totalItems ?? 0;
  const hasPrev = peersData?.meta?.hasPrev ?? false;
  const hasNext = peersData?.meta?.hasNext ?? false;
  const activeOnPage = peersData?.data?.filter((p) => p.active).length ?? 0;
  const rows = table.getRowModel().rows;

  if (!peersLoading && !peersError && peersData && total === 0) {
    return (
      <Empty className="border border-dashed">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <UsersIcon />
          </EmptyMedia>
          <EmptyTitle>No peers on this node</EmptyTitle>
          <EmptyDescription>
            Each client config you generate adds a peer here. Generate the first one to connect a
            device.
          </EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <Button onClick={onNewConfig}>
            <FilePlus2Icon />
            New config
          </Button>
        </EmptyContent>
      </Empty>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <InputGroup className="w-full sm:w-72">
          <InputGroupInput
            placeholder="Filter this page by ID, IP or key"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="Filter peers on this page"
            spellCheck={false}
            autoComplete="off"
          />
          <InputGroupAddon>
            <SearchIcon />
          </InputGroupAddon>
        </InputGroup>
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={activity}
          onValueChange={(value) => value && setActivity(value)}
          aria-label="Filter by activity"
        >
          <ToggleGroupItem value="all" className="px-3">
            All
          </ToggleGroupItem>
          <ToggleGroupItem value="active" className="px-3">
            Active
          </ToggleGroupItem>
          <ToggleGroupItem value="inactive" className="px-3">
            Inactive
          </ToggleGroupItem>
        </ToggleGroup>
        <span className="ml-auto text-sm text-muted-foreground tabular-nums max-sm:hidden">
          {plural(total, 'peer')} · {activeOnPage} active on this page
        </span>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="outline"
              size="icon-sm"
              aria-label="Reload peers"
              disabled={peersLoading}
              onClick={() => setReloadKey((k) => k + 1)}
            >
              <RefreshCwIcon className={cn(peersLoading && 'animate-spin')} />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Reload peers</TooltipContent>
        </Tooltip>
      </div>

      {peersError ? (
        <Alert variant="destructive">
          <CircleAlertIcon />
          <AlertTitle>Peers could not be loaded</AlertTitle>
          <AlertDescription>
            <p>{peersError}</p>
            <Button
              variant="outline"
              size="sm"
              className="mt-2"
              onClick={() => setReloadKey((k) => k + 1)}
            >
              Try again
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      <div className="overflow-x-auto rounded-lg border">
        <Table>
          <TableHeader className="bg-muted/50">
            {table.getHeaderGroups().map((group) => (
              <TableRow key={group.id} className="hover:bg-transparent">
                {group.headers.map((header) => {
                  const sorted = header.column.getIsSorted();
                  return (
                    <TableHead
                      key={header.id}
                      aria-sort={
                        sorted === false ? undefined : sorted === 'asc' ? 'ascending' : 'descending'
                      }
                      className={cn(
                        'h-9 text-xs',
                        header.id === 'actions' && 'w-12',
                        responsiveClass(header.id),
                      )}
                    >
                      {flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  );
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {peersLoading && !peersData ? (
              SKELETON_ROWS.map((key) => (
                <TableRow key={key}>
                  {columns.map((column) => (
                    <TableCell key={column.id} className={responsiveClass(column.id ?? '')}>
                      {column.id === 'actions' ? null : <Skeleton className="h-4 w-20" />}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : rows.length === 0 ? (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={columns.length} className="h-24 text-center">
                  <span className="text-muted-foreground">No peers on this page match.</span>{' '}
                  <Button
                    variant="link"
                    className="h-auto p-0"
                    onClick={() => patchParams({ pq: null, pfilter: null })}
                  >
                    Clear filters
                  </Button>
                </TableCell>
              </TableRow>
            ) : (
              rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={detailPeerId === row.original.peerId ? 'selected' : undefined}
                  className="cursor-pointer"
                  onClick={() => setDetailPeerId(row.original.peerId)}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell
                      key={cell.id}
                      className={cn('py-2.5', responsiveClass(cell.column.id))}
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

      <div className="flex flex-wrap items-center justify-between gap-4 text-sm">
        <span className="text-muted-foreground tabular-nums">
          {total > 0 ? `${offset + 1}–${Math.min(offset + limit, total)} of ${total}` : 'No peers'}
        </span>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground max-sm:hidden">Rows per page</span>
            <Select
              value={String(limit)}
              onValueChange={(value) =>
                patchParams({ plimit: value === DEFAULT_LIMIT ? null : value, poffset: null })
              }
            >
              <SelectTrigger size="sm" className="w-20" aria-label="Rows per page">
                <SelectValue />
              </SelectTrigger>
              <SelectContent align="end">
                {PAGE_SIZES.map((size) => (
                  <SelectItem key={size} value={String(size)}>
                    {size}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon-sm"
              aria-label="Previous page"
              disabled={!hasPrev}
              onClick={() => setOffset(peersData?.meta.prevOffset ?? Math.max(0, offset - limit))}
            >
              <ChevronLeftIcon />
            </Button>
            <Button
              variant="outline"
              size="icon-sm"
              aria-label="Next page"
              disabled={!hasNext}
              onClick={() => setOffset(peersData?.meta.nextOffset ?? offset + limit)}
            >
              <ChevronRightIcon />
            </Button>
          </div>
        </div>
      </div>

      <AlertDialog
        open={confirmDeleteId !== null}
        onOpenChange={(open) => {
          if (!open) setConfirmDeleteId(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete this peer?</AlertDialogTitle>
            <AlertDialogDescription>
              Peer{' '}
              <span className="font-mono text-foreground">{shortId(confirmDeleteId ?? '')}</span> is
              removed from the node and its config stops working. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Cancel</AlertDialogCancel>
            <Button
              variant="destructive"
              disabled={isDeleting}
              onClick={() => {
                if (confirmDeleteId !== null) void handleDeletePeer(confirmDeleteId);
              }}
            >
              {isDeleting ? <Spinner /> : null}
              Delete peer
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {detailPeerId ? (
        <PeerDetailSheet
          key={detailPeerId}
          nodeId={nodeId}
          peerId={detailPeerId}
          now={loadedAt}
          apiFetch={apiFetch}
          onClose={() => setDetailPeerId(null)}
          onDelete={setConfirmDeleteId}
        />
      ) : null}
    </div>
  );
};
