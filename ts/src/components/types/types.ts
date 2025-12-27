import { VectorClock } from '../clock/vector_clock';

// EntryType specifies the kind of data stored.
export enum EntryType {
  Memory = 'MEMORY',
  Auth = 'AUTH',
}

// DistributedDocument augments a document with CRDT metadata
export interface DistributedDocument {
  id: string;
  entryType: EntryType;
  payload?: Record<string, any>;
  _vector: VectorClock;
  _timestamp: number;
  _peerId: string;
  // Stage is an optional marker used to indicate special handling for a document.
  // Supported values: "post-pending" (document is staged and will be posted as a KNIRVGRAPH
  // transaction during the next sync), or empty string for normal documents.
  _stage?: string;
  _deleted?: boolean;
}

// OperationType enumerates CRDT operation kinds
export enum OperationType {
  Insert,
  Update,
  Delete,
}

// CRDTOperation represents a change to be synchronized
export interface CRDTOperation {
  id: string;
  type: OperationType;
  collection: string;
  documentId: string;
  data?: DistributedDocument;
  vector: VectorClock;
  timestamp: number;
  peerId: string;
}

// NetworkConfig holds network-level configuration
// Additional fields control posting/staging behavior:
//   - DefaultPostingNetwork: the network to which staged entries are posted (e.g. "knirvgraph").
//   - AutoPostClassifications: a list of EntryTypes that are automatically staged for posting by classification.
//   - PrivateByDefault: when true (default), entries are private unless staged or explicitly configured.
export interface NetworkConfig {
  networkId: string;
  name: string;
  collections: Record<string, boolean>;
  bootstrapPeers: string[];
  // Default posting target for staged entries (e.g., "knirvgraph").
  defaultPostingNetwork: string;

  // Entry classifications which are auto-staged for posting. Common defaults include
  // EntryType values like "ERROR", "CONTEXT", and "IDEA".
  autoPostClassifications: EntryType[];

  // Entries are private by default unless staged or configured otherwise.
  privateByDefault: boolean;

  encryption: {
    enabled: boolean;
    sharedSecret: string;
  };
  replication: {
    factor: number;
    strategy: string; // full | partial | leader
  };
  discovery: {
    mdns: boolean;
    bootstrap: boolean;
  };
}

// PeerInfo
export interface PeerInfo {
  peerId: string;
  addrs: string[];
  protocols: string[];
  latency: number; // in milliseconds
  lastSeen: Date;
  collections: string[];
}

// SyncState for a collection/network
export interface SyncState {
  collection: string;
  networkId: string;
  localVector: VectorClock;
  lastSync: Date;
  pendingOperations: CRDTOperation[];
  // StagedEntries contains IDs of documents marked with `_stage == "post-pending"`.
  // These will be converted to KNIRVGRAPH transactions and posted during the next sync.
  stagedEntries: string[];
  syncInProgress: boolean;
}

// NetworkStats
export interface NetworkStats {
  networkId: string;
  connectedPeers: number;
  totalPeers: number;
  collectionsShared: number;
  operationsSent: number;
  operationsReceived: number;
  bytesTransferred: number;
  averageLatency: number; // in milliseconds
}

// MessageType strings for protocol
export enum MessageType {
  SyncRequest = 'sync_request',
  SyncResponse = 'sync_response',
  Operation = 'operation',
  Heartbeat = 'heartbeat',
  CollectionAnnounce = 'collection_announce',
  CollectionRequest = 'collection_request',
}

// ProtocolMessage generic envelope
export interface ProtocolMessage {
  type: MessageType;
  networkId: string;
  senderId: string;
  timestamp: number;
  payload: any;
}