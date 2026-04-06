import { useCallback, useEffect, useState } from 'react';
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
import { renderConfig } from '@/lib/config-utils';
import { fetchErrorMessage } from '@/lib/api-client';
import { formatBytes, formatDate, formatExpires, truncate, PAGE_SIZES } from '@/lib/peer-utils';
import { usePeerDetail } from '@/hooks/usePeerDetail';
import type { PeersResponse } from '@/types';

type Props = {
  nodeId: string;
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  /** When this value changes, peers list is refetched (e.g. after config generated) */
  refreshTrigger?: number;
};

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

  const handleDeletePeer = async (peerId: string) => {
    setIsDeleting(true);
    setPeersError('');
    try {
      const res = await apiFetch(
        `/api/nodes/${encodeURIComponent(nodeId)}/peers?peerId=${encodeURIComponent(peerId)}`,
        { method: 'DELETE' },
      );
      if (res.ok) {
        if (detailPeerId === peerId) {
          setDetailPeerId(null);
        }
        setDeletePeerId(null);
        void loadPeers();
      } else {
        setPeersError('Failed to delete peer.');
      }
    } catch (err) {
      setPeersError(fetchErrorMessage(err) ?? 'Failed to delete peer.');
    } finally {
      setIsDeleting(false);
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
      className={`flex flex-col gap-4 transition-[margin] duration-200 ${isDetailsOpen ? 'mr-[24rem]' : ''}`}
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
            onClick={() => void loadPeers()}
            disabled={peersLoading}
          >
            <RefreshCw className={`size-4 ${peersLoading ? 'animate-spin' : ''}`} />
          </Button>
          <select
            className="h-8 rounded-md border border-input bg-background px-2 text-sm text-muted-foreground"
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
          <AlertCircle className="size-4" />
          <AlertDescription>{peersError}</AlertDescription>
        </Alert>
      )}

      <div className="overflow-x-auto overflow-y-hidden rounded-lg border shadow-xs">
        <Table className="min-w-[800px]">
          <TableHeader>
            <TableRow className="bg-muted/40 hover:bg-muted/40">
              <TableHead className="w-[160px] font-semibold">Peer ID</TableHead>
              <TableHead className="font-semibold">Allowed IPs</TableHead>
              <TableHead className="font-semibold">Public key</TableHead>
              <TableHead className="w-[100px] font-semibold">Status</TableHead>
              <TableHead className="font-semibold whitespace-nowrap">Last handshake</TableHead>
              <TableHead className="font-semibold">
                <button
                  type="button"
                  className="inline-flex items-center gap-1 text-sm font-semibold hover:text-foreground"
                  onClick={() => setSortDirection((prev) => (prev === 'desc' ? 'asc' : 'desc'))}
                >
                  <span>Created</span>
                  <ArrowUpDown className="size-3.5" />
                </button>
              </TableHead>
              <TableHead className="font-semibold">Expires</TableHead>
              <TableHead className="w-[100px] text-right font-semibold">Actions</TableHead>
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
                    <Skeleton className="h-5 w-16 rounded-full" />
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
              <TableRow>
                <TableCell colSpan={8} className="h-24 text-center text-muted-foreground">
                  No peers on this node
                </TableCell>
              </TableRow>
            ) : (
              sortedPeers.map((peer) => (
                <TableRow
                  key={peer.peerId}
                  className={`transition-colors ${
                    detailPeerId === peer.peerId
                      ? 'border-l-4 border-l-primary bg-primary/5'
                      : 'hover:bg-muted/30'
                  }`}
                >
                  <TableCell className="font-mono text-xs" title={peer.peerId}>
                    {truncate(peer.peerId, 22)}
                  </TableCell>
                  <TableCell className="font-mono text-sm" title={peer.allowedIPs?.join(', ')}>
                    {peer.allowedIPs?.join(', ') ?? '—'}
                  </TableCell>
                  <TableCell
                    className="max-w-[140px] truncate font-mono text-xs"
                    title={peer.publicKey}
                  >
                    {truncate(peer.publicKey, 24)}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={peer.active ? 'default' : 'secondary'}
                      className={peer.active ? 'bg-emerald-600 hover:bg-emerald-700' : ''}
                    >
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
                        className="size-8 text-destructive hover:bg-destructive/10 hover:text-destructive"
                        title="Delete peer"
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
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {(detailPeerId || detailLoading) && (
        <>
          <div
            className="fixed inset-0 z-40 bg-black/20 backdrop-blur-sm"
            aria-hidden
            onClick={() => {
              setDetailPeerId(null);
            }}
          />
          <div className="fixed top-0 right-0 z-50 flex h-full w-[22rem] flex-col border-l border-border bg-background shadow-2xl">
            {/* Header */}
            <div className="flex items-center justify-between border-b px-5 py-4">
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
                <div className="flex flex-col divide-y">
                  {/* Allowed IPs */}
                  <div className="flex flex-col gap-1 px-5 py-3">
                    <p className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                      Allowed IPs
                    </p>
                    <p className="font-mono text-sm">{detail.allowedIPs?.join(', ') ?? '—'}</p>
                  </div>

                  {/* Address families */}
                  {(detail.addressFamilies?.length ?? 0) > 0 && (
                    <div className="flex flex-col gap-1 px-5 py-3">
                      <p className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                        Address families
                      </p>
                      <p className="text-sm">{detail.addressFamilies?.join(', ')}</p>
                    </div>
                  )}

                  {/* Public key */}
                  <div className="flex flex-col gap-1 px-5 py-3">
                    <p className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                      Public key
                    </p>
                    <p
                      className="overflow-x-auto font-mono text-xs whitespace-nowrap text-muted-foreground"
                      title={detail.publicKey}
                    >
                      {detail.publicKey}
                    </p>
                  </div>

                  {/* Status grid */}
                  <div className="grid grid-cols-2 divide-x">
                    <div className="flex flex-col gap-1 px-5 py-3">
                      <p className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                        Status
                      </p>
                      <Badge
                        variant={detail.active ? 'default' : 'secondary'}
                        className={`w-fit ${detail.active ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300' : ''}`}
                      >
                        {detail.active ? 'Active' : 'Inactive'}
                      </Badge>
                    </div>
                    <div className="flex flex-col gap-1 px-5 py-3">
                      <p className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                        Last handshake
                      </p>
                      <p className="text-sm">{formatDate(detail.lastHandshakeAt)}</p>
                    </div>
                  </div>

                  {/* Created / Expires */}
                  <div className="grid grid-cols-2 divide-x">
                    <div className="flex flex-col gap-1 px-5 py-3">
                      <p className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                        Created
                      </p>
                      <p className="text-sm">{formatDate(detail.createdAt)}</p>
                    </div>
                    <div className="flex flex-col gap-1 px-5 py-3">
                      <p className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                        Expires
                      </p>
                      <p className="text-sm">{formatExpires(detail.expiresAt)}</p>
                    </div>
                  </div>

                  {/* Traffic */}
                  {(typeof detail.receiveBytes === 'number' ||
                    typeof detail.transmitBytes === 'number') && (
                    <div className="grid grid-cols-2 divide-x">
                      {typeof detail.receiveBytes === 'number' && (
                        <div className="flex flex-col gap-1 px-5 py-3">
                          <p className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                            ↓ Receive
                          </p>
                          <p className="font-mono text-sm font-medium">
                            {formatBytes(detail.receiveBytes)}
                          </p>
                        </div>
                      )}
                      {typeof detail.transmitBytes === 'number' && (
                        <div className="flex flex-col gap-1 px-5 py-3">
                          <p className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                            ↑ Transmit
                          </p>
                          <p className="font-mono text-sm font-medium">
                            {formatBytes(detail.transmitBytes)}
                          </p>
                        </div>
                      )}
                    </div>
                  )}

                  {/* Config */}
                  {(configError || configText) && (
                    <div className="px-5 py-3">
                      {configError && (
                        <Alert variant="destructive">
                          <AlertCircle className="size-4" />
                          <AlertDescription>{configError}</AlertDescription>
                        </Alert>
                      )}
                      {configText && (
                        <pre className="max-h-56 overflow-auto rounded-md bg-muted/50 px-3 py-2 font-mono text-xs leading-relaxed">
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
              <div className="flex items-center justify-between gap-2 border-t px-5 py-4">
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    className="gap-1.5"
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
                  className="gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive"
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
