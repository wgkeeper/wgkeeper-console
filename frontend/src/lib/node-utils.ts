/** Host and port of a node address, without the scheme: "fra-1.example.net:51821". */
export function nodeHost(address: string): string {
  try {
    return new URL(address).host;
  } catch {
    return address;
  }
}

/** The node's API scheme in upper case, or null when the address does not parse. */
export function nodeScheme(address: string): string | null {
  try {
    return new URL(address).protocol.replace(':', '').toUpperCase();
  } catch {
    return null;
  }
}

/** First block of a UUID, enough to tell nodes and peers apart in a list. */
export const shortId = (id: string) => id.split('-')[0] ?? id;
