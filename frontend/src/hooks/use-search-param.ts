import { useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';

/**
 * One URL search parameter as state. Setting it keeps every other parameter
 * and replaces the history entry; the default value is left out of the URL.
 */
export function useSearchParam(name: string, defaultValue = '') {
  const [params, setParams] = useSearchParams();
  const value = params.get(name) ?? defaultValue;

  const setValue = useCallback(
    (next: string) => {
      setParams(
        (prev) => {
          const merged = new URLSearchParams(prev);
          if (next === defaultValue || next === '') merged.delete(name);
          else merged.set(name, next);
          return merged;
        },
        { replace: true },
      );
    },
    [name, defaultValue, setParams],
  );

  return [value, setValue] as const;
}

/**
 * Sets several search parameters in one navigation. Two setters from
 * useSearchParam called in the same event would each start from the same
 * URL, and the second would undo the first.
 */
export function usePatchSearchParams() {
  const [, setParams] = useSearchParams();
  return useCallback(
    (patch: Record<string, string | null>) => {
      setParams(
        (prev) => {
          const merged = new URLSearchParams(prev);
          for (const [key, value] of Object.entries(patch)) {
            if (value === null || value === '') merged.delete(key);
            else merged.set(key, value);
          }
          return merged;
        },
        { replace: true },
      );
    },
    [setParams],
  );
}
