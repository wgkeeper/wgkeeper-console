import { useRef, useState } from 'react';
import { CircleAlertIcon, FileCodeIcon, ShuffleIcon } from 'lucide-react';
import { toast } from 'sonner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group';
import { Skeleton } from '@/components/ui/skeleton';
import { Spinner } from '@/components/ui/spinner';
import { ConfigOutput } from '@/components/nodes/ConfigOutput';
import { fetchErrorMessage, rateLimitMessage } from '@/lib/api-client';
import { shortId } from '@/lib/node-utils';
import { generateUUID, isValidDnsValue, isValidUuidV4 } from '@/lib/validation';

type AddressFamily = 'IPv4' | 'IPv6';

type Props = {
  nodeId: string;
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  onPeersChanged?: () => void;
  onShowPeers?: () => void;
  supportedAddressFamilies?: string[];
  isStatsLoading?: boolean;
};

const ALL_FAMILIES: readonly AddressFamily[] = ['IPv4', 'IPv6'];

const isPast = (iso: string) => new Date(iso).getTime() <= Date.now();

const filterSupported = (supported?: string[]): AddressFamily[] => {
  if (!supported || supported.length === 0) return [...ALL_FAMILIES];
  return ALL_FAMILIES.filter((f) => supported.includes(f));
};

export const NodeConfigTab = ({
  nodeId,
  apiFetch,
  onPeersChanged,
  onShowPeers,
  supportedAddressFamilies,
  isStatsLoading,
}: Props) => {
  const [configPeerId, setConfigPeerId] = useState(() => generateUUID());
  const [configDns1, setConfigDns1] = useState('1.1.1.1');
  const [configDns2, setConfigDns2] = useState('1.0.0.1');
  const [configDns3, setConfigDns3] = useState('2606:4700:4700::1111');
  const [configDns4, setConfigDns4] = useState('2606:4700:4700::1001');
  const [configExpiresAt, setConfigExpiresAt] = useState('');
  const [configExpiresAtTime, setConfigExpiresAtTime] = useState('');
  const renderedFamilies = filterSupported(supportedAddressFamilies);
  const supportsIPv4 = renderedFamilies.includes('IPv4');
  const supportsIPv6 = renderedFamilies.includes('IPv6');
  const [selectedFamilies, setConfigAddressFamilies] = useState<AddressFamily[]>([...ALL_FAMILIES]);
  // Derived, not synced: the selection limited to what the node supports, or
  // everything it supports when that leaves nothing, so the form is never empty.
  const supportedSelection = selectedFamilies.filter((f) => renderedFamilies.includes(f));
  const configAddressFamilies =
    supportedSelection.length > 0 ? supportedSelection : renderedFamilies;
  const outputRef = useRef<HTMLDivElement>(null);
  const [configText, setConfigText] = useState('');
  const [configPeer, setConfigPeer] = useState('');
  const [configError, setConfigError] = useState('');
  const [isConfigLoading, setIsConfigLoading] = useState(false);

  const handleFetchConfig = async (event: React.SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setConfigError('');
    setConfigText('');

    const peerId = configPeerId.trim();
    if (!peerId) {
      setConfigError('Enter a peer ID.');
      return;
    }
    if (!isValidUuidV4(peerId)) {
      setConfigError('The peer ID must be a UUID v4.');
      return;
    }

    if (configAddressFamilies.length === 0) {
      setConfigError('Select at least one address family.');
      return;
    }

    const ipv4Dns = supportsIPv4 ? [configDns1, configDns2] : [];
    const ipv6Dns = supportsIPv6 ? [configDns3, configDns4] : [];
    const dnsValues = [...ipv4Dns, ...ipv6Dns].map((v) => v.trim()).filter(Boolean);
    if (dnsValues.some((v) => !isValidDnsValue(v))) {
      setConfigError('Each DNS server must be an IPv4 or IPv6 address.');
      return;
    }

    const query = new URLSearchParams({ peerId });
    const dns = dnsValues.join(',');
    if (dns) query.set('dns', dns);

    const expiresAtDateRaw = configExpiresAt.trim();
    const expiresAtTimeRaw = configExpiresAtTime.trim();
    if (expiresAtDateRaw) {
      let expiresAt: string;
      // Both forms are local time: a date alone means the end of that day.
      const localDate = new Date(`${expiresAtDateRaw}T${expiresAtTimeRaw || '23:59:59'}`);
      if (Number.isNaN(localDate.getTime())) {
        setConfigError('The expiry date or time is not valid.');
        return;
      }
      expiresAt = localDate.toISOString();
      if (isPast(expiresAt)) {
        setConfigError('The expiry must be in the future.');
        return;
      }
      query.set('expiresAt', expiresAt);
    }
    if (configAddressFamilies.length > 0) {
      query.set('addressFamilies', configAddressFamilies.join(','));
    }

    setIsConfigLoading(true);
    let res: Response;
    try {
      // POST: generating a config creates or rotates the peer on the node.
      res = await apiFetch(`/api/nodes/${encodeURIComponent(nodeId)}/config?${query.toString()}`, {
        method: 'POST',
      });
    } catch (err) {
      setConfigError(fetchErrorMessage(err) ?? 'The node did not return a config.');
      setIsConfigLoading(false);
      return;
    }
    if (!res.ok) {
      const rl = rateLimitMessage(res);
      if (rl) {
        setConfigError(rl);
        setIsConfigLoading(false);
        return;
      }
      try {
        const body = (await res.json()) as {
          error?: string;
          status?: number;
          endpoint?: string;
          errorCode?: string;
          errorMessage?: string;
        };
        const topError = body?.error;
        const code = body?.errorCode;
        const msg = body?.errorMessage?.trim();
        if (topError === 'invalid_api_key') {
          setConfigError("The node no longer accepts this console's API key.");
        } else if (topError === 'incomplete_peer_response') {
          setConfigError('The node returned an incomplete peer record.');
        } else if (msg) {
          setConfigError(msg);
        } else if (code === 'invalid_expires_at') {
          setConfigError('The expiry must be in the future.');
        } else if (code === 'invalid_peer_id') {
          setConfigError('The peer ID must be a UUID v4.');
        } else if (code === 'unsupported_address_family') {
          setConfigError('This node does not support the selected address family.');
        } else if (code === 'no_available_ip') {
          setConfigError('The node has no free addresses left in its subnets.');
        } else if (code === 'server_info_unavailable' || code === 'wireguard_error') {
          setConfigError(msg || 'The node or WireGuard reported an error. Try again later.');
        } else {
          setConfigError('The node did not return a config.');
        }
      } catch {
        setConfigError('The node did not return a config.');
      }
      setIsConfigLoading(false);
      return;
    }
    let text: string;
    try {
      text = await res.text();
    } catch (err) {
      setConfigError(fetchErrorMessage(err) ?? 'The node did not return a config.');
      setIsConfigLoading(false);
      return;
    }
    setConfigText(text);
    setConfigPeer(peerId);
    setConfigPeerId(generateUUID());
    setIsConfigLoading(false);
    // On narrow screens the output stacks under the form, out of view.
    requestAnimationFrame(() => {
      const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
      outputRef.current?.scrollIntoView({
        behavior: reduceMotion ? 'auto' : 'smooth',
        block: 'nearest',
      });
      outputRef.current?.focus({ preventScroll: true });
    });
    toast.success('Config generated', {
      description: `Peer ${shortId(peerId)} is on the node.`,
      action: onShowPeers ? { label: 'View peers', onClick: onShowPeers } : undefined,
    });
    onPeersChanged?.();
  };

  return (
    <div className="grid items-start gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>
            <h2>New client config</h2>
          </CardTitle>
          <CardDescription>
            Creates a peer on this node and returns its WireGuard config. Reusing a peer ID rotates
            that peer’s keys.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form id="generate-config-form" onSubmit={handleFetchConfig}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="config-peer-id">Peer ID</FieldLabel>
                <InputGroup>
                  <InputGroupInput
                    id="config-peer-id"
                    className="font-mono text-[0.8125rem]"
                    placeholder="xxxxxxxx-xxxx-4xxx-xxxx-xxxxxxxxxxxx"
                    value={configPeerId}
                    onChange={(e) => setConfigPeerId(e.target.value)}
                    spellCheck={false}
                    autoComplete="off"
                    autoCapitalize="none"
                    required
                  />
                  <InputGroupAddon align="inline-end">
                    <InputGroupButton
                      size="xs"
                      onClick={() => setConfigPeerId(generateUUID())}
                      aria-label="Generate a new peer ID"
                    >
                      <ShuffleIcon />
                      New
                    </InputGroupButton>
                  </InputGroupAddon>
                </InputGroup>
                <FieldDescription>A UUID v4 you can map to a user or device.</FieldDescription>
              </Field>

              <Field>
                <FieldLabel htmlFor="config-expires-at">Expires</FieldLabel>
                <div className="grid grid-cols-2 gap-2">
                  <Input
                    id="config-expires-at"
                    type="date"
                    value={configExpiresAt}
                    onChange={(e) => setConfigExpiresAt(e.target.value)}
                  />
                  <Input
                    id="config-expires-at-time"
                    type="time"
                    aria-label="Expiry time"
                    value={configExpiresAtTime}
                    onChange={(e) => setConfigExpiresAtTime(e.target.value)}
                  />
                </div>
                <FieldDescription>
                  Optional, in your local time. The node removes the peer then. A date alone means
                  the end of that day.
                </FieldDescription>
              </Field>

              {isStatsLoading ? (
                <div className="flex flex-col gap-2">
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-9 w-full" />
                </div>
              ) : renderedFamilies.length > 1 ? (
                <FieldSet>
                  <FieldLegend variant="label">Address families</FieldLegend>
                  <div className="flex gap-6">
                    {renderedFamilies.map((family) => {
                      const checked = configAddressFamilies.includes(family);
                      // Lock the checkbox if it's the only one currently selected:
                      // unchecking it would leave the form in an invalid empty state.
                      const isLastChecked = checked && configAddressFamilies.length === 1;
                      return (
                        <Field key={family} orientation="horizontal" className="w-auto">
                          <Checkbox
                            id={`family-${family}`}
                            checked={checked}
                            disabled={isLastChecked}
                            onCheckedChange={(value) =>
                              setConfigAddressFamilies((prev) =>
                                value === true
                                  ? [...prev, family]
                                  : prev.filter((f) => f !== family),
                              )
                            }
                          />
                          <FieldLabel htmlFor={`family-${family}`} className="font-normal">
                            {family}
                          </FieldLabel>
                        </Field>
                      );
                    })}
                  </div>
                </FieldSet>
              ) : null}

              {isStatsLoading ? (
                <div className="flex flex-col gap-2">
                  <Skeleton className="h-4 w-24" />
                  <Skeleton className="h-9 w-full" />
                </div>
              ) : (
                <FieldSet>
                  <FieldLegend variant="label">DNS servers</FieldLegend>
                  <FieldDescription>
                    Cloudflare is filled in. Clear the fields to omit DNS.
                  </FieldDescription>
                  <div className="flex flex-col gap-2">
                    {supportsIPv4 ? (
                      <div className="grid grid-cols-2 gap-2">
                        <Input
                          aria-label="IPv4 DNS server 1"
                          spellCheck={false}
                          autoComplete="off"
                          inputMode="decimal"
                          className="font-mono text-[0.8125rem]"
                          placeholder="1.1.1.1"
                          value={configDns1}
                          onChange={(e) => setConfigDns1(e.target.value)}
                        />
                        <Input
                          aria-label="IPv4 DNS server 2"
                          spellCheck={false}
                          autoComplete="off"
                          inputMode="decimal"
                          className="font-mono text-[0.8125rem]"
                          placeholder="1.0.0.1"
                          value={configDns2}
                          onChange={(e) => setConfigDns2(e.target.value)}
                        />
                      </div>
                    ) : null}
                    {supportsIPv6 ? (
                      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                        <Input
                          aria-label="IPv6 DNS server 1"
                          spellCheck={false}
                          autoComplete="off"
                          className="font-mono text-[0.8125rem]"
                          placeholder="2606:4700:4700::1111"
                          value={configDns3}
                          onChange={(e) => setConfigDns3(e.target.value)}
                        />
                        <Input
                          aria-label="IPv6 DNS server 2"
                          spellCheck={false}
                          autoComplete="off"
                          className="font-mono text-[0.8125rem]"
                          placeholder="2606:4700:4700::1001"
                          value={configDns4}
                          onChange={(e) => setConfigDns4(e.target.value)}
                        />
                      </div>
                    ) : null}
                  </div>
                </FieldSet>
              )}

              {configError ? (
                <Alert variant="destructive">
                  <CircleAlertIcon />
                  <AlertTitle>Config not generated</AlertTitle>
                  <AlertDescription>{configError}</AlertDescription>
                </Alert>
              ) : null}
            </FieldGroup>
          </form>
        </CardContent>
        <CardFooter className="border-t">
          <Button type="submit" form="generate-config-form" disabled={isConfigLoading}>
            {isConfigLoading ? <Spinner /> : null}
            Generate config
          </Button>
        </CardFooter>
      </Card>

      {configText ? (
        <div
          ref={outputRef}
          tabIndex={-1}
          className="flex scroll-mt-20 flex-col gap-3 outline-none"
          aria-label="Generated config"
        >
          <Alert>
            <FileCodeIcon />
            <AlertTitle>Save this config now</AlertTitle>
            <AlertDescription>
              It holds the peer’s private key, which the console does not keep. Import it into the
              WireGuard app or download the file.
            </AlertDescription>
          </Alert>
          <ConfigOutput config={configText} fileName={`wgkeeper-${shortId(configPeer)}.conf`} />
        </div>
      ) : (
        <Empty className="h-full min-h-64 border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <FileCodeIcon />
            </EmptyMedia>
            <EmptyTitle>The config appears here</EmptyTitle>
            <EmptyDescription>
              Fill in the form and generate. You can copy the config or download it as a .conf file.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      )}
    </div>
  );
};
