export const REQUEST_TIMEOUT_MS = 30_000;

/**
 * Converts a fetch error into a user-facing message.
 * Returns null for AbortError (component unmounted — silently ignore).
 */
export function fetchErrorMessage(err: unknown): string | null {
  if (err instanceof DOMException) {
    if (err.name === 'AbortError') return null;
    if (err.name === 'TimeoutError') return 'Request timed out.';
  }
  if (err instanceof TypeError) return 'Network error. Check your connection.';
  return 'Request failed. Try again.';
}

/**
 * Creates a fetch wrapper that:
 * - Uses cookie-based auth via credentials=include
 * - Applies a per-request timeout (default 30 s); merges with caller's AbortSignal
 * - Calls onUnauthenticated() on 401 responses
 *
 * Throws on network errors and timeouts — callers must handle these.
 */
export function createApiFetch(onUnauthenticated: () => void, timeoutMs = REQUEST_TIMEOUT_MS) {
  const withTimeout = (existing?: AbortSignal): AbortSignal => {
    const timeout = AbortSignal.timeout(timeoutMs);
    return existing ? AbortSignal.any([existing, timeout]) : timeout;
  };

  const buildHeaders = (options: RequestInit): Headers => {
    const headers = new Headers(options.headers);
    if (!headers.has('Content-Type') && options.body) {
      headers.set('Content-Type', 'application/json');
    }
    return headers;
  };

  return async (path: string, options: RequestInit = {}): Promise<Response> => {
    const res = await fetch(path, {
      ...options,
      headers: buildHeaders(options),
      credentials: 'include',
      signal: withTimeout(options.signal ?? undefined),
    });

    if (res.status === 401) {
      onUnauthenticated();
    }

    return res;
  };
}
