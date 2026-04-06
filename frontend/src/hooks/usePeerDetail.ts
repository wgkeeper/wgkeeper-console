import { useCallback, useEffect, useRef, useState } from 'react';
import { fetchErrorMessage } from '@/lib/api-client';
import type { PeerDetail } from '@/types';

type ApiFetch = (path: string, options?: RequestInit) => Promise<Response>;

export type PeerDetailState = {
  detail: PeerDetail | null;
  detailLoading: boolean;
  configText: string;
  configError: string;
  isConfigLoading: boolean;
  configCopied: boolean;
  handleRegenerateConfig: () => Promise<void>;
  handleCopyConfig: () => Promise<void>;
};

export function usePeerDetail(
  nodeId: string,
  peerId: string | null,
  apiFetch: ApiFetch,
): PeerDetailState {
  const [detail, setDetail] = useState<PeerDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [configText, setConfigText] = useState('');
  const [configError, setConfigError] = useState('');
  const [isConfigLoading, setIsConfigLoading] = useState(false);
  const [configCopied, setConfigCopied] = useState(false);
  const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
    },
    [],
  );

  // Load peer detail whenever peerId changes
  useEffect(() => {
    if (!peerId || !nodeId) {
      setDetail(null);
      setConfigText('');
      setConfigError('');
      setConfigCopied(false);
      return;
    }
    let cancelled = false;
    setDetailLoading(true);
    apiFetch(`/api/nodes/${encodeURIComponent(nodeId)}/peers/${encodeURIComponent(peerId)}`)
      .then((res) => {
        if (cancelled) return;
        if (!res.ok) {
          setDetail(null);
          return;
        }
        return res.json() as Promise<PeerDetail>;
      })
      .then((data) => {
        if (cancelled) return;
        if (data) setDetail(data);
        setDetailLoading(false);
      })
      .catch(() => {
        if (!cancelled) setDetailLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [nodeId, peerId, apiFetch]);

  const handleRegenerateConfig = useCallback(async () => {
    if (!peerId || !nodeId) return;
    setConfigError('');
    setConfigText('');
    setConfigCopied(false);
    setIsConfigLoading(true);
    try {
      const res = await apiFetch(
        `/api/nodes/${encodeURIComponent(nodeId)}/config?peerId=${encodeURIComponent(peerId)}`,
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
          } else if (code === 'invalid_peer_id') {
            setConfigError('peerId must be a valid UUID v4.');
          } else if (code === 'no_available_ip') {
            setConfigError('No free IP in the node pool.');
          } else if (code === 'server_info_unavailable' || code === 'wireguard_error') {
            setConfigError(msg || 'Node or WireGuard error. Try again later.');
          } else {
            const parts: string[] = [];
            if (body?.status) parts.push(`node ${body.status}`);
            if (body?.endpoint) parts.push(body.endpoint);
            setConfigError(
              `Config is unavailable${res.status ? ` (${res.status})` : ''}${
                parts.length ? ` (${parts.join(', ')})` : ''
              }.`,
            );
          }
        } catch {
          setConfigError('Config is unavailable.');
        }
        return;
      }
      setConfigText(await res.text());
    } catch (err) {
      setConfigError(fetchErrorMessage(err) ?? 'Config is unavailable.');
    } finally {
      setIsConfigLoading(false);
    }
  }, [nodeId, peerId, apiFetch]);

  const handleCopyConfig = useCallback(async () => {
    if (!configText) return;
    try {
      await navigator.clipboard.writeText(configText);
      setConfigCopied(true);
      if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
      copyTimerRef.current = setTimeout(() => setConfigCopied(false), 1500);
    } catch {
      setConfigError('Failed to copy config.');
    }
  }, [configText]);

  return {
    detail,
    detailLoading,
    configText,
    configError,
    isConfigLoading,
    configCopied,
    handleRegenerateConfig,
    handleCopyConfig,
  };
}
