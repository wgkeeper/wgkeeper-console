import { describe, it, expect } from 'vitest';
import { formatBytes, formatDate, formatRelative, plural, PAGE_SIZES } from './peer-utils';

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
  it('returns em dash for a missing date', () => {
    expect(formatDate(null)).toBe('—');
    expect(formatDate('')).toBe('—');
  });

  it('formats a valid ISO string in the runtime locale', () => {
    // Noon UTC keeps the calendar day the same in every UTC-12..UTC+12 zone.
    const result = formatDate('2024-06-15T12:00:00.000Z');
    expect(result).toContain('2024');
    expect(result).toContain('15');
  });

  it('returns the original string if date is invalid', () => {
    expect(formatDate('not-a-date')).toBe('not-a-date');
  });
});

describe('formatRelative', () => {
  const now = Date.parse('2026-01-10T12:00:00Z');

  it('says just now for the last few seconds', () => {
    expect(formatRelative('2026-01-10T11:59:55Z', now)).toBe('just now');
  });

  it('picks the largest whole unit, past and future', () => {
    expect(formatRelative('2026-01-10T11:57:00Z', now)).toBe('3 min. ago');
    expect(formatRelative('2026-01-12T12:00:00Z', now)).toBe('in 2 days');
  });

  it('handles missing and invalid dates', () => {
    expect(formatRelative(null, now)).toBe('—');
    expect(formatRelative('bad-date', now)).toBe('bad-date');
  });
});

describe('plural', () => {
  it('uses the singular only for one', () => {
    expect(plural(1, 'peer')).toBe('1 peer');
    expect(plural(0, 'peer')).toBe('0 peers');
    expect(plural(3, 'entry', 'entries')).toBe('3 entries');
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
