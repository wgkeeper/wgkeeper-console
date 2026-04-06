import { describe, it, expect } from 'vitest';
import { formatBytes, formatDate, formatExpires, truncate, PAGE_SIZES } from './peer-utils';

describe('formatBytes', () => {
  it('formats bytes', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(512)).toBe('512 B');
    expect(formatBytes(1023)).toBe('1023 B');
  });

  it('formats kilobytes', () => {
    expect(formatBytes(1024)).toBe('1.0 KB');
    expect(formatBytes(1536)).toBe('1.5 KB');
    expect(formatBytes(1024 * 1023)).toBe('1023.0 KB');
  });

  it('formats megabytes', () => {
    expect(formatBytes(1024 * 1024)).toBe('1.0 MB');
    expect(formatBytes(1024 * 1024 * 50)).toBe('50.0 MB');
  });

  it('formats gigabytes', () => {
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1.0 GB');
    expect(formatBytes(1024 * 1024 * 1024 * 2.5)).toBe('2.5 GB');
  });
});

describe('formatDate', () => {
  it('returns em dash for null', () => {
    expect(formatDate(null)).toBe('—');
  });

  it('formats a valid ISO string (checks date part only — time shifts with timezone)', () => {
    // Use noon UTC so the date stays stable across all UTC-12..UTC+12 offsets
    const result = formatDate('2024-06-15T12:00:00.000Z');
    expect(result).toMatch(/15\.06\.2024/);
  });

  it('returns the original string if date is invalid', () => {
    expect(formatDate('not-a-date')).toBe('not-a-date');
  });
});

describe('formatExpires', () => {
  it('returns "No limit" for null', () => {
    expect(formatExpires(null)).toBe('No limit');
  });

  it('returns "No limit" for undefined', () => {
    expect(formatExpires(undefined)).toBe('No limit');
  });

  it('returns "No limit" for empty string', () => {
    expect(formatExpires('')).toBe('No limit');
  });

  it('formats a valid ISO string (checks date part only — time shifts with timezone)', () => {
    // Use noon UTC so the date stays stable across all UTC-12..UTC+12 offsets
    const result = formatExpires('2025-06-15T12:00:00.000Z');
    expect(result).toMatch(/15\.06\.2025/);
  });

  it('returns the original string if date is invalid', () => {
    expect(formatExpires('bad-date')).toBe('bad-date');
  });
});

describe('truncate', () => {
  it('returns the string unchanged if within limit', () => {
    expect(truncate('hello', 10)).toBe('hello');
    expect(truncate('hello', 5)).toBe('hello');
  });

  it('truncates and appends ellipsis if over limit', () => {
    expect(truncate('hello world', 5)).toBe('hello…');
    expect(truncate('abcdef', 3)).toBe('abc…');
  });

  it('handles empty string', () => {
    expect(truncate('', 5)).toBe('');
  });
});

describe('PAGE_SIZES', () => {
  it('is an ascending list of positive integers', () => {
    for (let i = 1; i < PAGE_SIZES.length; i++) {
      expect(PAGE_SIZES[i]).toBeGreaterThan(PAGE_SIZES[i - 1] as number);
    }
  });

  it('contains reasonable page sizes', () => {
    expect(PAGE_SIZES).toContain(25);
    expect(PAGE_SIZES).toContain(50);
  });
});
