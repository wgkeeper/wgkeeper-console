import { useState } from 'react';
import { toast } from 'sonner';
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
import { Spinner } from '@/components/ui/spinner';
import { rateLimitMessage } from '@/lib/api-client';
import { nodeHost } from '@/lib/node-utils';
import type { NodeItem } from '@/types';

type Props = {
  node: NodeItem | null;
  onOpenChange: (open: boolean) => void;
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  onDeleted: () => Promise<void> | void;
};

export const DeleteNodeDialog = ({ node, onOpenChange, apiFetch, onDeleted }: Props) => {
  const [isDeleting, setIsDeleting] = useState(false);

  const handleDelete = async () => {
    if (!node) return;
    setIsDeleting(true);
    try {
      const res = await apiFetch(`/api/nodes/${encodeURIComponent(node.id)}`, {
        method: 'DELETE',
      });
      // 404 means it is already gone: reconcile the list either way.
      if (res.ok || res.status === 404) {
        onOpenChange(false);
        toast.success('Node removed', { description: nodeHost(node.address) });
        await onDeleted();
        return;
      }
      toast.error(rateLimitMessage(res) ?? 'Could not remove the node. Try again.');
    } catch {
      toast.error('Network error. Try again.');
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <AlertDialog open={node !== null} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Remove this node?</AlertDialogTitle>
          <AlertDialogDescription>
            <span className="font-mono text-foreground">{node ? nodeHost(node.address) : ''}</span>{' '}
            and its stored API key are removed from the console. The node itself, its WireGuard
            interface and its peers keep working.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={isDeleting}>Cancel</AlertDialogCancel>
          <Button variant="destructive" disabled={isDeleting} onClick={() => void handleDelete()}>
            {isDeleting ? <Spinner /> : null}
            Remove node
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
};
