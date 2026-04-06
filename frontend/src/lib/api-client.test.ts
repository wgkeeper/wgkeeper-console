import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createApiFetch, fetchErrorMessage } from './api-client';

const mockFetch = vi.fn<typeof fetch>();
vi.stubGlobal('fetch', mockFetch);

const ok = (body: unknown, status = 200): Response =>
  ({ ok: true, status, json: () => Promise.resolve(body) }) as Response;

const fail = (status: number): Response =>
  ({ ok: false, status, json: () => Promise.resolve({}) }) as Response;

beforeEach(() => {
  mockFetch.mockReset();
});

describe('createApiFetch', () => {
  describe('Content-Type header', () => {
    it('sets application/json when body is present and Content-Type is not set', async () => {
      mockFetch.mockResolvedValueOnce(ok({}));
      const apiFetch = createApiFetch(vi.fn());

      await apiFetch('/api/nodes', { method: 'POST', body: '{}' });

      const [, init] = mockFetch.mock.calls[0]!;
      expect(new Headers(init?.headers).get('Content-Type')).toBe('application/json');
    });

    it('does not override explicit Content-Type', async () => {
      mockFetch.mockResolvedValueOnce(ok({}));
      const apiFetch = createApiFetch(vi.fn());

      await apiFetch('/api/upload', {
        method: 'POST',
        body: 'data',
        headers: { 'Content-Type': 'text/plain' },
      });

      const [, init] = mockFetch.mock.calls[0]!;
      expect(new Headers(init?.headers).get('Content-Type')).toBe('text/plain');
    });

    it('does not set Content-Type when there is no body', async () => {
      mockFetch.mockResolvedValueOnce(ok({}));
      const apiFetch = createApiFetch(vi.fn());

      await apiFetch('/api/nodes');

      const [, init] = mockFetch.mock.calls[0]!;
      expect(new Headers(init?.headers).get('Content-Type')).toBeNull();
    });
  });

  describe('401 handling', () => {
    it('calls onUnauthenticated and returns the 401 response', async () => {
      mockFetch.mockResolvedValueOnce(fail(401));
      const onUnauthenticated = vi.fn();
      const apiFetch = createApiFetch(onUnauthenticated);

      const res = await apiFetch('/api/nodes');

      expect(onUnauthenticated).toHaveBeenCalledOnce();
      expect(res.status).toBe(401);
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });

    it('does not retry on non-401 errors', async () => {
      mockFetch.mockResolvedValueOnce(fail(500));
      const apiFetch = createApiFetch(vi.fn());

      const res = await apiFetch('/api/nodes');

      expect(res.status).toBe(500);
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });
  });

  describe('error propagation', () => {
    it('propagates network error (TypeError) to the caller', async () => {
      const networkError = new TypeError('Failed to fetch');
      mockFetch.mockRejectedValueOnce(networkError);
      const apiFetch = createApiFetch(vi.fn());

      await expect(apiFetch('/api/nodes')).rejects.toThrow('Failed to fetch');
    });

    it('propagates timeout error (DOMException TimeoutError) to the caller', async () => {
      const timeoutError = new DOMException('Request timed out', 'TimeoutError');
      mockFetch.mockRejectedValueOnce(timeoutError);
      const apiFetch = createApiFetch(vi.fn());

      await expect(apiFetch('/api/nodes')).rejects.toThrow('Request timed out');
    });

    it('propagates AbortError to the caller', async () => {
      const abortError = new DOMException('Aborted', 'AbortError');
      mockFetch.mockRejectedValueOnce(abortError);
      const apiFetch = createApiFetch(vi.fn());

      await expect(apiFetch('/api/nodes')).rejects.toThrow('Aborted');
    });

    it('does not call onUnauthenticated on network error', async () => {
      mockFetch.mockRejectedValueOnce(new TypeError('Failed to fetch'));
      const onUnauthenticated = vi.fn();
      const apiFetch = createApiFetch(onUnauthenticated);

      await apiFetch('/api/nodes').catch(() => {});

      expect(onUnauthenticated).not.toHaveBeenCalled();
    });
  });
});

describe('fetchErrorMessage', () => {
  it('returns null for AbortError (component unmount — silent)', () => {
    expect(fetchErrorMessage(new DOMException('Aborted', 'AbortError'))).toBeNull();
  });

  it('returns timeout message for TimeoutError', () => {
    expect(fetchErrorMessage(new DOMException('Timeout', 'TimeoutError'))).toBe(
      'Request timed out.',
    );
  });

  it('returns network error message for TypeError', () => {
    expect(fetchErrorMessage(new TypeError('Failed to fetch'))).toBe(
      'Network error. Check your connection.',
    );
  });

  it('returns generic message for unknown errors', () => {
    expect(fetchErrorMessage(new Error('Something went wrong'))).toBe('Request failed. Try again.');
    expect(fetchErrorMessage('string error')).toBe('Request failed. Try again.');
  });
});
