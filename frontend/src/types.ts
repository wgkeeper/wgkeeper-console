export type NodeListResponse = {
  nodes: NodeItem[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
};

export type NodeItem = {
  id: string; // UUID, main identifier
  name: string;
  address: string; // full base URL, e.g. https://192.168.1.1:51821
  status: 'online' | 'offline' | (string & {});
  version: string | null;
  isOutdated: boolean;
  latestVersion: string | null;
  latestVersionUrl: string | null;
  createdAt: string;
  updatedAt: string;
};

export type NodeCheckResult = {
  ok: boolean;
  status: string;
  version: string | null;
  serviceName: string | null;
  address: string;
};

export type NodeStats = {
  service?: {
    name?: string;
    version?: string;
  };
  wireguard?: {
    interface?: string;
    listenPort?: number;
    subnets?: string[];
    serverIps?: string[];
    addressFamilies?: string[];
  };
  peers?: {
    possible?: number;
    issued?: number;
    active?: number;
  };
  startedAt?: string;
};

export type PeerListItem = {
  peerId: string;
  allowedIPs: string[];
  addressFamilies?: string[];
  publicKey: string;
  active: boolean;
  lastHandshakeAt: string | null;
  createdAt: string;
  /** Optional expiration date (ISO string). Omitted or null = no time limit. */
  expiresAt?: string | null;
};

export type PeerDetail = PeerListItem & {
  receiveBytes?: number;
  transmitBytes?: number;
};

export type PeersMeta = {
  offset: number;
  limit: number;
  totalItems: number;
  hasPrev: boolean;
  hasNext: boolean;
  prevOffset: number | null;
  nextOffset: number | null;
};

export type PeersResponse = { data: PeerListItem[]; meta: PeersMeta };
