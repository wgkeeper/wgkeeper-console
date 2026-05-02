import { useCallback, useEffect, useRef, useState } from 'react';
import { fetchErrorMessage, rateLimitMessage } from '@/lib/api-client';
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
  const regenerateAbortRef = useRef<AbortController | null>(null);

  useEffect(
    () => () => {
      if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
      regenerateAbortRef.current?.abort();
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
    const controller = new AbortController();
    setDetailLoading(true);
    apiFetch(`/api/nodes/${encodeURIComponent(nodeId)}/peers/${encodeURIComponent(peerId)}`, {
      signal: controller.signal,
    })
      .then((res) => {
        if (controller.signal.aborted) return;
        if (!res.ok) {
          setDetail(null);
          return;
        }
        return res.json() as Promise<PeerDetail>;
      })
      .then((data) => {
        if (controller.signal.aborted) return;
        if (data) setDetail(data);
        setDetailLoading(false);
      })
      .catch((err: unknown) => {
        if ((err as Error).name === 'AbortError') return;
        setDetailLoading(false);
      });
    return () => controller.abort();
  }, [nodeId, peerId, apiFetch]);

  const handleRegenerateConfig = useCallback(async () => {
    if (!peerId || !nodeId) return;
    // Cancel any previous in-flight regenerate so a stale response can't
    // overwrite a fresh one (and unmount aborts via the cleanup effect).
    regenerateAbortRef.current?.abort();
    const controller = new AbortController();
    regenerateAbortRef.current = controller;
    setConfigError('');
    setConfigText('');
    setConfigCopied(false);
    setIsConfigLoading(true);
    try {
      const res = await apiFetch(
        `/api/nodes/${encodeURIComponent(nodeId)}/config?peerId=${encodeURIComponent(peerId)}`,
        { signal: controller.signal },
      );
      if (!res.ok) {
        const rl = rateLimitMessage(res);
        if (rl) {
          setConfigError(rl);
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
            setConfigError("This node's stored X_API_KEY is no longer valid.");
          } else if (topError === 'incomplete_peer_response') {
            setConfigError('The node returned an incomplete peer record.');
          } else if (msg) {
            setConfigError(msg);
          } else if (code === 'invalid_peer_id') {
            setConfigError('peerId must be a valid UUID v4.');
          } else if (code === 'no_available_ip') {
            setConfigError('No free IP in the node pool.');
          } else if (code === 'server_info_unavailable' || code === 'wireguard_error') {
            setConfigError(msg || 'Node or WireGuard error. Try again later.');
          } else {
            setConfigError('Config is unavailable.');
          }
        } catch {
          setConfigError('Config is unavailable.');
        }
        return;
      }
      setConfigText(await res.text());
    } catch (err) {
      if ((err as Error).name === 'AbortError') return;
      setConfigError(fetchErrorMessage(err) ?? 'Config is unavailable.');
    } finally {
      if (!controller.signal.aborted) setIsConfigLoading(false);
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
