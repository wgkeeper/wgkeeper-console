import { describe, it, expect } from 'vitest';
import {
  isValidUuidV4,
  isValidIpv4,
  isValidIpv6,
  isValidDnsValue,
  generateUUID,
} from './validation';

describe('isValidUuidV4', () => {
  it('accepts valid v4 UUIDs', () => {
    expect(isValidUuidV4('550e8400-e29b-41d4-a716-446655440000')).toBe(true);
    expect(isValidUuidV4('f47ac10b-58cc-4372-a567-0e02b2c3d479')).toBe(true);
    expect(isValidUuidV4('ffffffff-ffff-4fff-bfff-ffffffffffff')).toBe(true);
  });

  it('accepts uppercase UUIDs', () => {
    expect(isValidUuidV4('550E8400-E29B-41D4-A716-446655440000')).toBe(true);
  });

  it('trims whitespace before checking', () => {
    expect(isValidUuidV4('  550e8400-e29b-41d4-a716-446655440000  ')).toBe(true);
  });

  it('rejects wrong version (v1, v3, v5)', () => {
    expect(isValidUuidV4('550e8400-e29b-11d4-a716-446655440000')).toBe(false); // v1
    expect(isValidUuidV4('550e8400-e29b-31d4-a716-446655440000')).toBe(false); // v3
    expect(isValidUuidV4('550e8400-e29b-51d4-a716-446655440000')).toBe(false); // v5
  });

  it('rejects invalid variant byte (must be 8, 9, a, or b)', () => {
    expect(isValidUuidV4('550e8400-e29b-41d4-c716-446655440000')).toBe(false);
    expect(isValidUuidV4('550e8400-e29b-41d4-0716-446655440000')).toBe(false);
  });

  it('rejects missing hyphens', () => {
    expect(isValidUuidV4('550e8400e29b41d4a716446655440000')).toBe(false);
  });

  it('rejects empty string', () => {
    expect(isValidUuidV4('')).toBe(false);
  });

  it('rejects wrong length', () => {
    expect(isValidUuidV4('550e8400-e29b-41d4-a716')).toBe(false);
    expect(isValidUuidV4('550e8400-e29b-41d4-a716-4466554400001')).toBe(false);
  });
});

describe('isValidIpv4', () => {
  it('accepts valid IPv4 addresses', () => {
    expect(isValidIpv4('0.0.0.0')).toBe(true);
    expect(isValidIpv4('1.1.1.1')).toBe(true);
    expect(isValidIpv4('192.168.1.1')).toBe(true);
    expect(isValidIpv4('255.255.255.255')).toBe(true);
    expect(isValidIpv4('10.0.0.1')).toBe(true);
  });

  it('rejects out-of-range octets', () => {
    expect(isValidIpv4('256.0.0.0')).toBe(false);
    expect(isValidIpv4('1.1.1.300')).toBe(false);
  });

  it('rejects too few or too many octets', () => {
    expect(isValidIpv4('1.1.1')).toBe(false);
    expect(isValidIpv4('1.1.1.1.1')).toBe(false);
  });

  it('rejects non-numeric parts', () => {
    expect(isValidIpv4('a.b.c.d')).toBe(false);
    expect(isValidIpv4('1.1.1.x')).toBe(false);
  });

  it('rejects empty string', () => {
    expect(isValidIpv4('')).toBe(false);
  });
});

describe('isValidIpv6', () => {
  it('accepts valid IPv6 addresses', () => {
    expect(isValidIpv6('::1')).toBe(true);
    expect(isValidIpv6('2606:4700:4700::1111')).toBe(true);
    expect(isValidIpv6('fe80::1')).toBe(true);
    expect(isValidIpv6('2001:db8::1')).toBe(true);
  });

  it('rejects plain IPv4', () => {
    expect(isValidIpv6('1.1.1.1')).toBe(false);
  });

  it('rejects empty string', () => {
    expect(isValidIpv6('')).toBe(false);
  });

  it('rejects garbage', () => {
    expect(isValidIpv6('not-an-ip')).toBe(false);
    expect(isValidIpv6('gggg::1')).toBe(false);
  });
});

describe('isValidDnsValue', () => {
  it('accepts valid IPv4', () => {
    expect(isValidDnsValue('1.1.1.1')).toBe(true);
    expect(isValidDnsValue('8.8.8.8')).toBe(true);
  });

  it('accepts valid IPv6', () => {
    expect(isValidDnsValue('2606:4700:4700::1111')).toBe(true);
    expect(isValidDnsValue('::1')).toBe(true);
  });

  it('rejects hostnames (only IPs allowed for DNS)', () => {
    expect(isValidDnsValue('cloudflare.com')).toBe(false);
    expect(isValidDnsValue('dns.google')).toBe(false);
  });

  it('rejects empty string', () => {
    expect(isValidDnsValue('')).toBe(false);
  });
});

describe('generateUUID', () => {
  it('generates a valid v4 UUID', () => {
    const uuid = generateUUID();
    expect(isValidUuidV4(uuid)).toBe(true);
  });

  it('generates unique values', () => {
    const ids = new Set(Array.from({ length: 20 }, () => generateUUID()));
    expect(ids.size).toBe(20);
  });
});
