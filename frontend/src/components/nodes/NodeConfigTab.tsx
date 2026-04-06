import { useEffect, useRef, useState } from 'react';
import { AlertCircle, CheckCircle2, RefreshCw } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Separator } from '@/components/ui/separator';
import { renderConfig } from '@/lib/config-utils';
import { generateUUID, isValidDnsValue, isValidUuidV4 } from '@/lib/validation';

type Props = {
  nodeId: string;
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  onPeersChanged?: () => void;
};

export const NodeConfigTab = ({ nodeId, apiFetch, onPeersChanged }: Props) => {
  const [configPeerId, setConfigPeerId] = useState('');
  const [configDns1, setConfigDns1] = useState('1.1.1.1');
  const [configDns2, setConfigDns2] = useState('1.0.0.1');
  const [configDns3, setConfigDns3] = useState('2606:4700:4700::1111');
  const [configDns4, setConfigDns4] = useState('2606:4700:4700::1001');
  const [configExpiresAt, setConfigExpiresAt] = useState('');
  const [configExpiresAtTime, setConfigExpiresAtTime] = useState('');
  const [configAddressFamilies, setConfigAddressFamilies] = useState<('IPv4' | 'IPv6')[]>([
    'IPv4',
    'IPv6',
  ]);
  const [configText, setConfigText] = useState('');
  const [configError, setConfigError] = useState('');
  const [configNotice, setConfigNotice] = useState('');
  const [isConfigLoading, setIsConfigLoading] = useState(false);
  const [configCopied, setConfigCopied] = useState(false);
  const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
    },
    [],
  );

  const handleFetchConfig = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setConfigError('');
    setConfigNotice('');
    setConfigText('');
    setConfigCopied(false);

    const peerId = configPeerId.trim();
    if (!peerId) {
      setConfigError('Provide a peerId.');
      return;
    }
    if (!isValidUuidV4(peerId)) {
      setConfigError('peerId must be a valid UUID v4.');
      return;
    }

    const dnsValues = [configDns1, configDns2, configDns3, configDns4]
      .map((v) => v.trim())
      .filter(Boolean);
    if (dnsValues.length && dnsValues.some((v) => !isValidDnsValue(v))) {
      setConfigError('DNS must be a valid IPv4 or IPv6 address.');
      return;
    }

    const query = new URLSearchParams({ peerId });
    const dns = dnsValues.join(',');
    if (dns) query.set('dns', dns);

    const expiresAtDateRaw = configExpiresAt.trim();
    const expiresAtTimeRaw = configExpiresAtTime.trim();
    if (expiresAtDateRaw) {
      let expiresAt: string;
      if (expiresAtTimeRaw) {
        const localDate = new Date(`${expiresAtDateRaw}T${expiresAtTimeRaw}`);
        if (Number.isNaN(localDate.getTime())) {
          setConfigError('Invalid expiration date or time.');
          return;
        }
        expiresAt = localDate.toISOString();
      } else {
        expiresAt = `${expiresAtDateRaw}T23:59:59Z`;
      }
      if (new Date(expiresAt).getTime() <= Date.now()) {
        setConfigError('Expiration date and time must be in the future.');
        return;
      }
      query.set('expiresAt', expiresAt);
    }
    if (configAddressFamilies.length > 0) {
      query.set('addressFamilies', configAddressFamilies.join(','));
    }

    setIsConfigLoading(true);
    const res = await apiFetch(
      `/api/nodes/${encodeURIComponent(nodeId)}/config?${query.toString()}`,
    );
    if (!res.ok) {
      try {
        const body = (await res.json()) as {
          status?: number;
          endpoint?: string;
          errorCode?: string;
          errorMessage?: string;
        };
        const code = body?.errorCode;
        const msg = body?.errorMessage?.trim();
        if (msg) {
          setConfigError(msg);
        } else if (code === 'invalid_expires_at') {
          setConfigError('Expiration date must be in the future.');
        } else if (code === 'invalid_peer_id') {
          setConfigError('peerId must be a valid UUID v4.');
        } else if (code === 'unsupported_address_family') {
          setConfigError('Selected address family is not supported by this node.');
        } else if (code === 'no_available_ip') {
          setConfigError('No free IP in the node pool.');
        } else if (code === 'server_info_unavailable' || code === 'wireguard_error') {
          setConfigError(msg || 'Node or WireGuard error. Try again later.');
        } else {
          const parts = [];
          if (body?.status) parts.push(`node ${body.status}`);
          if (body?.endpoint) parts.push(body.endpoint);
          setConfigError(
            `Config is unavailable${res.status ? ` (${res.status})` : ''}${parts.length ? ` (${parts.join(', ')})` : ''}.`,
          );
        }
      } catch {
        setConfigError('Config is unavailable.');
      }
      setIsConfigLoading(false);
      return;
    }
    setConfigText(await res.text());
    setConfigNotice('Config generated. The peer is now on this node — see the Peers tab.');
    setIsConfigLoading(false);
    onPeersChanged?.();
  };

  const handleCopyConfig = async () => {
    if (!configText) return;
    try {
      await navigator.clipboard.writeText(configText);
      setConfigCopied(true);
      setConfigNotice('');
      if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
      copyTimerRef.current = setTimeout(() => setConfigCopied(false), 1500);
    } catch {
      setConfigError('Failed to copy config.');
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Generate config</CardTitle>
        <CardDescription>
          Creates a new peer on this node and returns a WireGuard client config.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form
          id="generate-config-form"
          className="flex flex-col gap-5"
          onSubmit={handleFetchConfig}
        >
          {/* Peer ID */}
          <div className="flex flex-col gap-1.5">
            <Label
              htmlFor="config-peer-id"
              className="text-xs font-medium tracking-wide text-muted-foreground uppercase"
            >
              Peer ID (UUID v4)
            </Label>
            <div className="flex gap-2">
              <Input
                id="config-peer-id"
                className="font-mono"
                placeholder="xxxxxxxx-xxxx-4xxx-xxxx-xxxxxxxxxxxx"
                value={configPeerId}
                onChange={(e) => setConfigPeerId(e.target.value)}
                required
              />
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="shrink-0"
                onClick={() => setConfigPeerId(generateUUID())}
              >
                Generate
              </Button>
            </div>
          </div>

          {/* Expiration */}
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
              Expires (optional)
            </Label>
            <div className="flex gap-2">
              <Input
                id="config-expires-at"
                type="date"
                value={configExpiresAt}
                onChange={(e) => setConfigExpiresAt(e.target.value)}
              />
              <Input
                id="config-expires-at-time"
                type="time"
                value={configExpiresAtTime}
                onChange={(e) => setConfigExpiresAtTime(e.target.value)}
              />
            </div>
            <p className="text-xs text-muted-foreground">
              Leave empty for no limit. Date only → end of day UTC.
            </p>
          </div>

          {/* Address families */}
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
              Address families (optional)
            </Label>
            <div className="flex gap-5">
              {(['IPv4', 'IPv6'] as const).map((family) => (
                <label key={family} className="flex cursor-pointer items-center gap-2">
                  <input
                    type="checkbox"
                    checked={configAddressFamilies.includes(family)}
                    onChange={(e) =>
                      setConfigAddressFamilies((prev) =>
                        e.target.checked ? [...prev, family] : prev.filter((f) => f !== family),
                      )
                    }
                    className="size-4 rounded border-input accent-primary"
                  />
                  <span className="text-sm">{family}</span>
                </label>
              ))}
            </div>
            <p className="text-xs text-muted-foreground">
              Leave both unchecked to use all families the node supports.
            </p>
          </div>

          {/* DNS */}
          <div className="flex flex-col gap-1.5">
            <div className="flex items-baseline justify-between">
              <Label className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                DNS
              </Label>
              <span className="text-xs text-muted-foreground">
                Cloudflare pre-filled · edit or clear
              </span>
            </div>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <div className="flex flex-col gap-1">
                <span className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                  IPv4
                </span>
                <div className="flex gap-2">
                  <Input
                    placeholder="1.1.1.1"
                    value={configDns1}
                    onChange={(e) => setConfigDns1(e.target.value)}
                  />
                  <Input
                    placeholder="1.0.0.1"
                    value={configDns2}
                    onChange={(e) => setConfigDns2(e.target.value)}
                  />
                </div>
              </div>
              <div className="flex flex-col gap-1">
                <span className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                  IPv6
                </span>
                <div className="flex gap-2">
                  <Input
                    placeholder="2606:4700:4700::1111"
                    value={configDns3}
                    onChange={(e) => setConfigDns3(e.target.value)}
                  />
                  <Input
                    placeholder="2606:4700:4700::1001"
                    value={configDns4}
                    onChange={(e) => setConfigDns4(e.target.value)}
                  />
                </div>
              </div>
            </div>
          </div>

          {configNotice && (
            <Alert className="border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">
              <CheckCircle2 className="size-4 !text-emerald-600 dark:!text-emerald-400" />
              <AlertDescription>{configNotice}</AlertDescription>
            </Alert>
          )}
          {configError && (
            <Alert variant="destructive">
              <AlertCircle className="size-4" />
              <AlertDescription>{configError}</AlertDescription>
            </Alert>
          )}
          {configText && (
            <pre className="max-h-64 overflow-auto rounded-lg border bg-muted/30 px-4 py-3 font-mono text-xs leading-relaxed">
              {renderConfig(configText)}
            </pre>
          )}
        </form>
      </CardContent>
      <Separator />
      <CardFooter className="flex items-center gap-2 pt-4">
        <Button
          type="submit"
          form="generate-config-form"
          disabled={isConfigLoading}
          className="gap-1.5"
        >
          {isConfigLoading && <RefreshCw className="size-3.5 animate-spin" />}
          {isConfigLoading ? 'Generating…' : 'Generate config'}
        </Button>
        {configText && (
          <Button type="button" variant="outline" onClick={handleCopyConfig}>
            {configCopied ? 'Copied!' : 'Copy'}
          </Button>
        )}
      </CardFooter>
    </Card>
  );
};
