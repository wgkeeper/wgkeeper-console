import { useCallback, useEffect, useRef, useState } from 'react';
import {
  ArrowUpDown,
  ChevronLeft,
  ChevronRight,
  Eye,
  EyeOff,
  RefreshCw,
  Trash2,
  Users,
  X,
  AlertCircle,
} from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
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
import { cn } from '@/lib/utils';
import { renderConfig } from '@/lib/config-utils';
import { fetchErrorMessage, rateLimitMessage } from '@/lib/api-client';
import { formatBytes, formatDate, formatExpires, truncate, PAGE_SIZES } from '@/lib/peer-utils';
import { usePeerDetail } from '@/hooks/usePeerDetail';
import type { PeersResponse } from '@/types';

type Props = {
  nodeId: string;
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  /** When this value changes, peers list is refetched (e.g. after config generated) */
  refreshTrigger?: number;
};

const DrawerField = ({ label, children }: { label: string; children: React.ReactNode }) => (
  <div className="flex flex-col gap-1 px-5 py-3">
    <p className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
      {label}
    </p>
    {children}
  </div>
);

export const NodePeersTab = ({ nodeId, apiFetch, refreshTrigger }: Props) => {
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState(50);
  const [peersData, setPeersData] = useState<PeersResponse | null>(null);
  const [peersLoading, setPeersLoading] = useState(false);
  const [peersError, setPeersError] = useState('');
  const [detailPeerId, setDetailPeerId] = useState<string | null>(null);
  const [deletePeerId, setDeletePeerId] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('desc');
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);

  const {
    detail,
    detailLoading,
    configText,
    configError,
    isConfigLoading,
    configCopied,
    handleRegenerateConfig,
    handleCopyConfig,
  } = usePeerDetail(nodeId, detailPeerId, apiFetch);

  const loadPeers = useCallback(
    async (signal?: AbortSignal) => {
      setPeersLoading(true);
      setPeersError('');
      try {
        const res = await apiFetch(
          `/api/nodes/${encodeURIComponent(nodeId)}/peers?offset=${offset}&limit=${limit}`,
          signal ? { signal } : undefined,
        );
        if (signal?.aborted) return;
        if (!res.ok) {
          setPeersData(null);
          setPeersError('Failed to load peers.');
          return;
        }
        const data = (await res.json()) as PeersResponse;
        setPeersData(data);
      } catch (err) {
        if ((err as Error).name === 'AbortError') return;
        setPeersData(null);
        setPeersError('Failed to load peers.');
      } finally {
        if (!signal?.aborted) setPeersLoading(false);
      }
    },
    [nodeId, offset, limit, apiFetch],
  );

  useEffect(() => {
    const controller = new AbortController();
    void loadPeers(controller.signal);
    return () => controller.abort();
  }, [loadPeers]);

  useEffect(() => {
    if (refreshTrigger != null && refreshTrigger > 0) {
      void loadPeers();
    }
  }, [refreshTrigger, loadPeers]);

  const deleteAbortRef = useRef<AbortController | null>(null);

  useEffect(() => () => deleteAbortRef.current?.abort(), []);

  const handleDeletePeer = async (peerId: string) => {
    deleteAbortRef.current?.abort();
    const controller = new AbortController();
    deleteAbortRef.current = controller;
    setIsDeleting(true);
    setPeersError('');
    try {
      const res = await apiFetch(
        `/api/nodes/${encodeURIComponent(nodeId)}/peers?peerId=${encodeURIComponent(peerId)}`,
        { method: 'DELETE', signal: controller.signal },
      );
      if (res.ok) {
        if (detailPeerId === peerId) {
          setDetailPeerId(null);
        }
        setDeletePeerId(null);
        void loadPeers();
      } else {
        setPeersError(rateLimitMessage(res) ?? 'Failed to delete peer.');
      }
    } catch (err) {
      if ((err as Error).name === 'AbortError') return;
      setPeersError(fetchErrorMessage(err) ?? 'Failed to delete peer.');
    } finally {
      if (!controller.signal.aborted) setIsDeleting(false);
    }
  };

  const total = peersData?.meta?.totalItems ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / limit));
  const currentPage = Math.floor(offset / limit) + 1;
  const hasPrev = peersData?.meta?.hasPrev ?? false;
  const hasNext = peersData?.meta?.hasNext ?? false;
  const activeOnPage = peersData?.data?.filter((p) => p.active).length ?? 0;

  const sortedPeers = peersData?.data
    ? [...peersData.data].sort((a, b) => {
        const aTime = a.createdAt ? new Date(a.createdAt).getTime() : 0;
        const bTime = b.createdAt ? new Date(b.createdAt).getTime() : 0;
        return sortDirection === 'desc' ? bTime - aTime : aTime - bTime;
      })
    : [];

  const isDetailsOpen = Boolean(detailPeerId || detailLoading);

  return (
    <div
      className={cn(
        'flex flex-col gap-4 transition-[margin] duration-200 ease-out',
        isDetailsOpen && 'mr-[24rem]',
      )}
    >
      <div className="flex flex-wrap items-center justify-between gap-4">
        <span className="flex items-center gap-1.5 text-sm font-medium">
          <Users className="size-4 text-muted-foreground" />
          {total} peer{total !== 1 ? 's' : ''}
          {total > 0 && (
            <span className="font-normal text-muted-foreground">
              {' · '}
              {activeOnPage} active
            </span>
          )}
        </span>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="icon"
            className="size-8"
            title={peersLoading ? 'Loading…' : 'Refresh'}
            aria-label="Refresh peers"
            onClick={() => void loadPeers()}
            disabled={peersLoading}
          >
            <RefreshCw className={`size-4 ${peersLoading ? 'animate-spin' : ''}`} />
          </Button>
          <select
            className="h-8 rounded-md border border-input bg-background px-2 text-sm text-muted-foreground transition-[color,border-color,box-shadow] duration-150 outline-none hover:border-ring/40 focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
            value={limit}
            title="Rows per page"
            onChange={(e) => {
              setLimit(Number(e.target.value));
              setOffset(0);
            }}
          >
            {PAGE_SIZES.map((s) => (
              <option key={s} value={s}>
                {s} / page
              </option>
            ))}
          </select>
          <div className="flex items-center">
            <Button
              variant="ghost"
              size="icon"
              className="size-8"
              aria-label="Previous page"
              disabled={!hasPrev}
              onClick={() => setOffset(peersData?.meta.prevOffset ?? Math.max(0, offset - limit))}
            >
              <ChevronLeft className="size-4" />
            </Button>
            <span className="min-w-[48px] text-center text-sm text-muted-foreground tabular-nums">
              {currentPage} / {totalPages}
            </span>
            <Button
              variant="ghost"
              size="icon"
              className="size-8"
              aria-label="Next page"
              disabled={!hasNext}
              onClick={() => setOffset(peersData?.meta.nextOffset ?? offset + limit)}
            >
              <ChevronRight className="size-4" />
            </Button>
          </div>
        </div>
      </div>

      {peersError && (
        <Alert variant="destructive">
          <AlertCircle />
          <AlertDescription>{peersError}</AlertDescription>
        </Alert>
      )}

      <div className="overflow-x-auto overflow-y-hidden rounded-lg border border-border">
        <Table className="min-w-[800px]">
          <TableHeader>
            <TableRow className="border-border bg-surface hover:bg-surface">
              <TableHead className="w-[160px]">Peer ID</TableHead>
              <TableHead>Allowed IPs</TableHead>
              <TableHead>Public key</TableHead>
              <TableHead className="w-[100px]">Status</TableHead>
              <TableHead className="whitespace-nowrap">Last handshake</TableHead>
              <TableHead>
                <button
                  type="button"
                  className="inline-flex items-center gap-1 text-xs font-medium transition-colors hover:text-foreground"
                  onClick={() => setSortDirection((prev) => (prev === 'desc' ? 'asc' : 'desc'))}
                >
                  <span>Created</span>
                  <ArrowUpDown className="size-3.5" />
                </button>
              </TableHead>
              <TableHead>Expires</TableHead>
              <TableHead className="w-[100px] text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {peersLoading && !peersData ? (
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell>
                    <Skeleton className="h-4 w-[130px]" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-24" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-[110px]" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-5 w-16 rounded-md" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-28" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-28" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-20" />
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      <Skeleton className="size-8 rounded-md" />
                      <Skeleton className="size-8 rounded-md" />
                    </div>
                  </TableCell>
                </TableRow>
              ))
            ) : sortedPeers.length === 0 ? (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={8} className="h-28 text-center">
                  <div className="flex flex-col items-center gap-1.5 text-muted-foreground">
                    <Users className="size-5" />
                    <span className="text-sm">No peers on this node yet</span>
                  </div>
                </TableCell>
              </TableRow>
            ) : (
              sortedPeers.map((peer) => (
                <TableRow
                  key={peer.peerId}
                  data-selected={detailPeerId === peer.peerId}
                  className="border-border transition-colors data-[selected=false]:hover:bg-accent/40 data-[selected=true]:bg-accent"
                >
                  <TableCell className="font-mono text-xs whitespace-nowrap" title={peer.peerId}>
                    {truncate(peer.peerId, 22)}
                  </TableCell>
                  <TableCell
                    className="font-mono text-sm whitespace-nowrap"
                    title={peer.allowedIPs?.join(', ')}
                  >
                    {peer.allowedIPs?.join(', ') ?? '—'}
                  </TableCell>
                  <TableCell
                    className="max-w-[140px] truncate font-mono text-xs text-muted-foreground"
                    title={peer.publicKey}
                  >
                    {truncate(peer.publicKey, 24)}
                  </TableCell>
                  <TableCell>
                    <Badge variant={peer.active ? 'success' : 'secondary'}>
                      {peer.active ? (
                        <span className="size-1.5 rounded-full bg-success" aria-hidden />
                      ) : null}
                      {peer.active ? 'Active' : 'Inactive'}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {formatDate(peer.lastHandshakeAt)}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {formatDate(peer.createdAt)}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {formatExpires(peer.expiresAt)}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-0.5">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-8"
                        title={detailPeerId === peer.peerId ? 'Hide details' : 'View details'}
                        aria-label={detailPeerId === peer.peerId ? 'Hide details' : 'View details'}
                        onClick={() =>
                          setDetailPeerId(detailPeerId === peer.peerId ? null : peer.peerId)
                        }
                      >
                        {detailPeerId === peer.peerId ? (
                          <EyeOff className="size-4" />
                        ) : (
                          <Eye className="size-4" />
                        )}
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-8 text-muted-foreground hover:bg-destructive-muted hover:text-destructive"
                        title="Delete peer"
                        aria-label="Delete peer"
                        disabled={isDeleting && deletePeerId === peer.peerId}
                        onClick={() => setConfirmDeleteId(peer.peerId)}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <AlertDialog
        open={confirmDeleteId !== null}
        onOpenChange={(open) => {
          if (!open) setConfirmDeleteId(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete peer?</AlertDialogTitle>
            <AlertDialogDescription>This action cannot be undone.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                if (!confirmDeleteId) return;
                setDeletePeerId(confirmDeleteId);
                void handleDeletePeer(confirmDeleteId);
                setConfirmDeleteId(null);
              }}
            >
              Delete peer
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {(detailPeerId || detailLoading) && (
        <>
          <div
            className="fixed inset-0 z-40 bg-black/40 backdrop-blur-[2px]"
            aria-hidden
            onClick={() => {
              setDetailPeerId(null);
            }}
          />
          <div className="fixed top-0 right-0 z-50 flex h-full w-[22rem] flex-col border-l border-border bg-background shadow-xl">
            {/* Header */}
            <div className="flex items-center justify-between border-b border-border px-5 py-4">
              <div className="flex min-w-0 flex-col gap-0.5">
                <h2 className="text-sm font-semibold">Peer details</h2>
                <p
                  className="truncate font-mono text-[11px] text-muted-foreground"
                  title={detailPeerId ?? undefined}
                >
                  {detailPeerId ?? 'Loading…'}
                </p>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="size-7 shrink-0"
                onClick={() => {
                  setDetailPeerId(null);
                }}
                title="Close"
                aria-label="Close details"
              >
                <X className="size-4" />
              </Button>
            </div>

            {/* Content */}
            <div className="flex-1 overflow-y-auto">
              {detailLoading ? (
                <div className="flex flex-col gap-4 p-5">
                  {Array.from({ length: 4 }).map((_, i) => (
                    <div key={i} className="flex flex-col gap-1.5">
                      <Skeleton className="h-3 w-20" />
                      <Skeleton className="h-8 w-full rounded-md" />
                    </div>
                  ))}
                </div>
              ) : detail ? (
                <div className="flex flex-col divide-y divide-border">
                  <DrawerField label="Allowed IPs">
                    <p className="font-mono text-sm">{detail.allowedIPs?.join(', ') ?? '—'}</p>
                  </DrawerField>

                  {(detail.addressFamilies?.length ?? 0) > 0 && (
                    <DrawerField label="Address families">
                      <p className="text-sm">{detail.addressFamilies?.join(', ')}</p>
                    </DrawerField>
                  )}

                  <DrawerField label="Public key">
                    <p
                      className="overflow-x-auto font-mono text-xs whitespace-nowrap text-muted-foreground"
                      title={detail.publicKey}
                    >
                      {detail.publicKey}
                    </p>
                  </DrawerField>

                  <div className="grid grid-cols-2 divide-x divide-border">
                    <DrawerField label="Status">
                      <Badge variant={detail.active ? 'success' : 'secondary'} className="w-fit">
                        {detail.active ? (
                          <span className="size-1.5 rounded-full bg-success" aria-hidden />
                        ) : null}
                        {detail.active ? 'Active' : 'Inactive'}
                      </Badge>
                    </DrawerField>
                    <DrawerField label="Last handshake">
                      <p className="text-sm">{formatDate(detail.lastHandshakeAt)}</p>
                    </DrawerField>
                  </div>

                  <div className="grid grid-cols-2 divide-x divide-border">
                    <DrawerField label="Created">
                      <p className="text-sm">{formatDate(detail.createdAt)}</p>
                    </DrawerField>
                    <DrawerField label="Expires">
                      <p className="text-sm">{formatExpires(detail.expiresAt)}</p>
                    </DrawerField>
                  </div>

                  {(typeof detail.receiveBytes === 'number' ||
                    typeof detail.transmitBytes === 'number') && (
                    <div className="grid grid-cols-2 divide-x divide-border">
                      {typeof detail.receiveBytes === 'number' && (
                        <DrawerField label="↓ Receive">
                          <p className="font-mono text-sm font-medium">
                            {formatBytes(detail.receiveBytes)}
                          </p>
                        </DrawerField>
                      )}
                      {typeof detail.transmitBytes === 'number' && (
                        <DrawerField label="↑ Transmit">
                          <p className="font-mono text-sm font-medium">
                            {formatBytes(detail.transmitBytes)}
                          </p>
                        </DrawerField>
                      )}
                    </div>
                  )}

                  {(configError || configText) && (
                    <div className="px-5 py-3">
                      {configError && (
                        <Alert variant="destructive">
                          <AlertCircle />
                          <AlertDescription>{configError}</AlertDescription>
                        </Alert>
                      )}
                      {configText && (
                        <pre className="max-h-56 overflow-auto rounded-md border border-border bg-muted/50 px-3 py-2 font-mono text-xs leading-relaxed">
                          {renderConfig(configText)}
                        </pre>
                      )}
                    </div>
                  )}
                </div>
              ) : (
                <p className="p-5 text-sm text-muted-foreground">Peer not found.</p>
              )}
            </div>

            {/* Footer */}
            {detail && (
              <div className="flex items-center justify-between gap-2 border-t border-border px-5 py-4">
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={isConfigLoading || !detail?.peerId}
                    onClick={handleRegenerateConfig}
                  >
                    <RefreshCw className={`size-3.5 ${isConfigLoading ? 'animate-spin' : ''}`} />
                    {isConfigLoading ? 'Loading…' : 'Regenerate'}
                  </Button>
                  {configText && (
                    <Button variant="outline" size="sm" onClick={handleCopyConfig}>
                      {configCopied ? 'Copied!' : 'Copy'}
                    </Button>
                  )}
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-muted-foreground hover:bg-destructive-muted hover:text-destructive"
                  disabled={isDeleting}
                  onClick={() => {
                    if (detail?.peerId) setConfirmDeleteId(detail.peerId);
                  }}
                >
                  <Trash2 className="size-3.5" />
                  {isDeleting && deletePeerId === detail?.peerId ? 'Deleting…' : 'Delete'}
                </Button>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
};
