import { useState } from 'react';
import { CircleCheckIcon, CircleXIcon, EyeIcon, EyeOffIcon } from 'lucide-react';
import { toast } from 'sonner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group';
import { Spinner } from '@/components/ui/spinner';
import { fetchErrorMessage, rateLimitMessage } from '@/lib/api-client';
import type { NodeCheckResult } from '@/types';

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  onAdded: () => Promise<unknown>;
};

const EMPTY_FORM = { address: '', apiKey: '' };

/** The API's `error` code from a failed response, or null when the body has none. */
const errorCode = async (res: Response) => {
  try {
    return ((await res.json()) as { error?: string }).error ?? null;
  } catch {
    return null;
  }
};

const checkError = async (res: Response) =>
  rateLimitMessage(res) ??
  ((await errorCode(res)) === 'invalid_api_key'
    ? 'The node rejected this API key.'
    : 'The node did not respond. Check the address.');

const addError = async (res: Response) => {
  const limited = rateLimitMessage(res);
  if (limited) return limited;
  const code = await errorCode(res);
  if (code === 'node_exists' || res.status === 409) {
    return 'A node with this address is already in the console.';
  }
  if (code === 'invalid_input') return 'The address or API key is not valid.';
  return 'Could not add the node. Try again.';
};

export const AddNodeDialog = ({ open, onOpenChange, apiFetch, onAdded }: Props) => {
  const [form, setForm] = useState(EMPTY_FORM);
  const [showKey, setShowKey] = useState(false);
  const [check, setCheck] = useState<NodeCheckResult | null>(null);
  const [error, setError] = useState('');
  const [isChecking, setIsChecking] = useState(false);
  const [isSaving, setIsSaving] = useState(false);

  const reset = () => {
    setForm(EMPTY_FORM);
    setCheck(null);
    setError('');
    setShowKey(false);
  };

  const update = (field: keyof typeof EMPTY_FORM, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }));
    setCheck(null);
    setError('');
  };

  const address = form.address.trim();
  const apiKey = form.apiKey.trim();
  const checked = check?.ok === true && check.address === address;

  /** Tests the connection; resolves to whether the node answered with this key. */
  const handleCheck = async () => {
    setError('');
    if (!address || !apiKey) {
      setError('Enter the node address and its API key.');
      return false;
    }
    setIsChecking(true);
    try {
      const res = await apiFetch('/api/nodes/check', {
        method: 'POST',
        body: JSON.stringify({ address, apiKey }),
      });
      if (!res.ok) {
        setCheck(null);
        setError(await checkError(res));
        return false;
      }
      const data = (await res.json()) as Omit<NodeCheckResult, 'address'>;
      setCheck({ ...data, address });
      if (!data.ok) setError('The node did not respond. Check the address.');
      return data.ok;
    } catch (err) {
      setError(fetchErrorMessage(err) ?? 'Network error. Try again.');
      return false;
    } finally {
      setIsChecking(false);
    }
  };

  const handleSubmit = async (event: React.SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    // Adding always tests first, so Enter in either field does the whole job.
    if (!checked && !(await handleCheck())) return;
    setIsSaving(true);
    try {
      const res = await apiFetch('/api/nodes', {
        method: 'POST',
        body: JSON.stringify({ address, apiKey }),
      });
      if (res.ok) {
        await onAdded();
        toast.success('Node added', { description: address });
        reset();
        onOpenChange(false);
        return;
      }
      setCheck(null);
      setError(await addError(res));
    } catch (err) {
      setError(fetchErrorMessage(err) ?? 'Network error. Try again.');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) reset();
        onOpenChange(next);
      }}
    >
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit} className="flex flex-col gap-6">
          <DialogHeader>
            <DialogTitle>Add node</DialogTitle>
            <DialogDescription>
              Connect a wgkeeper node through its API. The key is stored encrypted.
            </DialogDescription>
          </DialogHeader>

          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="node-address">Node address</FieldLabel>
              <Input
                id="node-address"
                name="address"
                type="url"
                inputMode="url"
                className="font-mono"
                placeholder="https://10.0.0.1:51821"
                autoComplete="off"
                autoCapitalize="none"
                spellCheck={false}
                value={form.address}
                onChange={(e) => update('address', e.target.value)}
                required
              />
              <FieldDescription>The node API URL, including the port.</FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="node-api-key">API key</FieldLabel>
              <InputGroup>
                <InputGroupInput
                  id="node-api-key"
                  className="font-mono"
                  name="apiKey"
                  type={showKey ? 'text' : 'password'}
                  autoComplete="off"
                  autoCapitalize="none"
                  spellCheck={false}
                  placeholder="auth.api_key from the node config"
                  value={form.apiKey}
                  onChange={(e) => update('apiKey', e.target.value)}
                  required
                />
                <InputGroupAddon align="inline-end">
                  <InputGroupButton
                    size="icon-xs"
                    className="pointer-coarse:size-8"
                    aria-label={showKey ? 'Hide API key' : 'Show API key'}
                    onClick={() => setShowKey((v) => !v)}
                  >
                    {showKey ? <EyeOffIcon /> : <EyeIcon />}
                  </InputGroupButton>
                </InputGroupAddon>
              </InputGroup>
            </Field>
          </FieldGroup>

          {checked ? (
            <Alert className="[&>svg]:text-success-foreground">
              <CircleCheckIcon />
              <AlertTitle>Connected</AlertTitle>
              <AlertDescription>
                {check.serviceName ?? 'wgkeeper-node'}
                {check.version ? ` ${check.version}` : ''} is reachable with this key.
              </AlertDescription>
            </Alert>
          ) : null}
          {error ? (
            <Alert variant="destructive">
              <CircleXIcon />
              <AlertTitle>{error}</AlertTitle>
            </Alert>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={isChecking || isSaving}
              onClick={() => void handleCheck()}
            >
              {isChecking ? <Spinner /> : null}
              Test connection
            </Button>
            <Button type="submit" disabled={isChecking || isSaving}>
              {isSaving ? <Spinner /> : null}
              Add node
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
