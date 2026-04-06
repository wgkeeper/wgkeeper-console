export const isValidUuidV4 = (value: string) =>
  /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value.trim());

export const isValidIpv4 = (value: string) => {
  const parts = value.split('.');
  if (parts.length !== 4) return false;
  return parts.every((part) => {
    if (!/^\d{1,3}$/.test(part)) return false;
    const num = Number(part);
    return num >= 0 && num <= 255;
  });
};

export const isValidIpv6 = (value: string) => {
  const s = value.trim();
  if (s.length < 2 || s.length > 39) return false;
  try {
    new URL(`http://[${s}]`);
    return true;
  } catch {
    return false;
  }
};

export const isValidDnsValue = (value: string) => isValidIpv4(value) || isValidIpv6(value);

export function generateUUID(): string {
  return crypto.randomUUID();
}
