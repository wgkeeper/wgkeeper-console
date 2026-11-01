import { useRef, useState } from 'react';
import { CircleAlertIcon, RotateCwIcon, Trash2Icon } from 'lucide-react';
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
import { Separator } from '@/components/ui/separator';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';
import { CopyButton } from '@/components/CopyButton';
import { ConfigOutput } from '@/components/nodes/ConfigOutput';
import { PeerExpires } from '@/components/nodes/PeerExpires';
import { PeerStatusBadge } from '@/components/nodes/NodeStatusBadge';
import { usePeerDetail } from '@/hooks/usePeerDetail';
import { shortId } from '@/lib/node-utils';
import { formatBytes, formatDate, formatRelative } from '@/lib/peer-utils';
import { cn } from '@/lib/utils';

// Stable keys for loading placeholders: there's no data to key them by yet.
const SKELETON_FIELDS = ['field-1', 'field-2', 'field-3', 'field-4', 'field-5'];

type Props = {
  nodeId: string;
  peerId: string;
  now: number;
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  onClose: () => void;
  onDelete: (peerId: string) => void;
};

/**
 * Details of one peer. The parent renders it with key={peerId}, so opening
 * another peer starts from a clean state and a rotation still in flight for
 * the previous one is aborted with it.
 */
export const PeerDetailSheet = ({ nodeId, peerId, now, apiFetch, onClose, onDelete }: Props) => {
  const state = usePeerDetail(nodeId, peerId, apiFetch);
  const [confirmRotate, setConfirmRotate] = useState(false);
  const titleRef = useRef<HTMLHeadingElement>(null);

  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SheetContent
        className="w-full gap-0 sm:max-w-md"
        // Land on the title rather than the first copy button, so screen
        // readers announce the sheet without popping a tooltip.
        onOpenAutoFocus={(event) => {
          event.preventDefault();
          titleRef.current?.focus();
        }}
      >
        <SheetHeader className="border-b">
          <SheetTitle ref={titleRef} tabIndex={-1} className="outline-none">
            Peer {shortId(peerId)}
          </SheetTitle>
          <SheetDescription className="flex items-center gap-1 font-mono text-xs">
            {peerId}
            <CopyButton value={peerId} label="Copy peer ID" className="size-6" />
          </SheetDescription>
        </SheetHeader>
        <DetailBody state={state} now={now} />
        {state.detail ? (
          <SheetFooter className="flex-row justify-between border-t">
            <Button
              variant="outline"
              disabled={state.isConfigLoading}
              onClick={() => setConfirmRotate(true)}
            >
              <RotateCwIcon className={cn(state.isConfigLoading && 'animate-spin')} />
              Rotate keys
            </Button>
            <Button
              variant="ghost"
              className="text-destructive hover:bg-destructive/10 hover:text-destructive"
              onClick={() => onDelete(peerId)}
            >
              <Trash2Icon />
              Delete
            </Button>
          </SheetFooter>
        ) : null}
      </SheetContent>

      <AlertDialog open={confirmRotate} onOpenChange={setConfirmRotate}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Rotate this peer’s keys?</AlertDialogTitle>
            <AlertDialogDescription>
              The node issues new keys for peer{' '}
              <span className="font-mono text-foreground">{shortId(peerId)}</span> and keeps its IP
              address. The config the device uses now stops connecting until it gets the new one.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <Button
              onClick={() => {
                setConfirmRotate(false);
                void state.handleRegenerateConfig();
              }}
            >
              Rotate keys
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Sheet>
  );
};

const DetailBody = ({ state, now }: { state: ReturnType<typeof usePeerDetail>; now: number }) => {
  const { detail, detailLoading, configText, configError } = state;

  if (detailLoading) {
    return (
      <div className="flex flex-col gap-5 p-4" aria-busy="true">
        {SKELETON_FIELDS.map((key) => (
          <div key={key} className="flex flex-col gap-1.5">
            <Skeleton className="h-3 w-20" />
            <Skeleton className="h-4 w-full" />
          </div>
        ))}
      </div>
    );
  }
  if (!detail) {
    return <p className="p-4 text-sm text-muted-foreground">This peer is no longer on the node.</p>;
  }

  return (
    <div className="flex flex-1 flex-col gap-5 overflow-y-auto p-4">
      <dl className="grid grid-cols-2 gap-x-4 gap-y-4 text-sm">
        <DetailField label="Status">
          <PeerStatusBadge active={detail.active} />
        </DetailField>
        <DetailField label="Last handshake">
          <DateValue iso={detail.lastHandshakeAt} empty="Never" />
        </DetailField>
        <DetailField label="Allowed IPs" className="col-span-2">
          <span className="font-mono text-[0.8125rem]">{detail.allowedIPs?.join(', ') ?? '—'}</span>
        </DetailField>
        <DetailField label="Public key" className="col-span-2">
          <span className="flex items-center gap-1">
            <span className="truncate font-mono text-xs">{detail.publicKey}</span>
            <CopyButton value={detail.publicKey} label="Copy public key" className="size-6" />
          </span>
        </DetailField>
        {typeof detail.receiveBytes === 'number' ? (
          <DetailField label="Received">
            <span className="font-mono">{formatBytes(detail.receiveBytes)}</span>
          </DetailField>
        ) : null}
        {typeof detail.transmitBytes === 'number' ? (
          <DetailField label="Sent">
            <span className="font-mono">{formatBytes(detail.transmitBytes)}</span>
          </DetailField>
        ) : null}
        <DetailField label="Created">
          <DateValue iso={detail.createdAt} empty="—" />
        </DetailField>
        <DetailField label="Expires">
          <PeerExpires iso={detail.expiresAt} now={now} />
          {detail.expiresAt ? (
            <span className="block text-xs text-muted-foreground">
              {formatDate(detail.expiresAt)}
            </span>
          ) : null}
        </DetailField>
        {(detail.addressFamilies?.length ?? 0) > 0 ? (
          <DetailField label="Address families">{detail.addressFamilies?.join(', ')}</DetailField>
        ) : null}
      </dl>

      {configError || configText ? <Separator /> : null}
      {configError ? (
        <Alert variant="destructive">
          <CircleAlertIcon />
          <AlertTitle>Keys not rotated</AlertTitle>
          <AlertDescription>{configError}</AlertDescription>
        </Alert>
      ) : null}
      {configText ? (
        <div className="flex flex-col gap-2">
          <p className="text-sm text-muted-foreground">
            Keys rotated and the IP address kept. Give the device this config; the previous one no
            longer connects.
          </p>
          <ConfigOutput config={configText} fileName={`wgkeeper-${shortId(detail.peerId)}.conf`} />
        </div>
      ) : null}
    </div>
  );
};

/** Relative time with the exact date under it, visible on touch screens too. */
const DateValue = ({ iso, empty }: { iso?: string | null; empty: string }) =>
  iso ? (
    <time dateTime={iso} className="flex flex-col">
      <span>{formatRelative(iso)}</span>
      <span className="text-xs text-muted-foreground">{formatDate(iso)}</span>
    </time>
  ) : (
    <span>{empty}</span>
  );

const DetailField = ({
  label,
  className,
  children,
}: {
  label: string;
  className?: string;
  children: React.ReactNode;
}) => (
  <div className={cn('flex min-w-0 flex-col gap-1', className)}>
    <dt className="text-xs text-muted-foreground">{label}</dt>
    <dd className="min-w-0">{children}</dd>
  </div>
);
