# NebulusDB Distributed Database Implementation

## Overview

This document outlines the implementation of NebulusDB as a distributed database using DHT (Distributed Hash Table) technology. The architecture enables peer-to-peer data synchronization across standalone application instances while maintaining all existing functionality.

## Architecture Goals

- **Local-First:** All operations work locally with background synchronization
- **Network Isolation:** Each peer network is an independent consortium with separate collections
- **Dynamic Membership:** Collections can be added/removed from networks at runtime
- **Conflict Resolution:** CRDT-based automatic conflict resolution with vector clocks
- **Type Safety:** Full TypeScript type safety throughout
- **Backward Compatible:** Existing non-distributed functionality remains unchanged

## Storage and Persistence

The KNIRV network utilizes a custom-built storage and persistence layer designed for flexibility and efficiency in a distributed environment. This layer implements the storage methods detailed in this document, with optimized strategies for both `MEMORY` and `AUTH` data entry types.

### Key Storage Features:

*   **Default Authentication:** Authentication handling is included by default to ensure secure access to the database. All `AUTH` type entries are stored using a simple, encrypted key-value model.
*   **File Storage Location:** All database files are stored in the standard OS application data directory (e.g., `~/.config` on Linux, `%APPDATA%` on Windows) to ensure proper separation and user-specific data management.
*   **`MEMORY` Entry Type Storage:** Entries of the `MEMORY` type are persisted using a multi-file structure on the **localhost filesystem** within the OS application data directory. This separation allows for efficient handling of large binary data while keeping metadata and vectors easily accessible. An IPFS-based storage solution for distributed blob storage is planned for a future release.

    For each `MEMORY` entry, the following are saved:
    1.  **Data Blob:** The raw data is saved as a separate file on the local system. 
    2.  **Vector Representation:** The vector embedding is stored alongside the metadata within the main database file/store. It is synchronized between peers.
    3.  **Metadata Facts Table:** The structured metadata is stored in the main database file/store and is synchronized between peers.

    The synchronized document (`DistributedDocument`) contains metadata about the blob (like its path or a content hash), but the blob data itself is **not** synchronized across the network in this version.


### Design Rationale: Why Blobs Are Not Synchronized

In this architecture, only the metadata and vector representation for `MEMORY` entries are synchronized across peers. The raw data blob is stored only on the local system where it was created. This design was chosen for several key reasons related to performance and efficiency in a distributed system:

1.  **Network Efficiency:** Broadcasting large binary files (blobs) to every peer upon every change would consume significant bandwidth, leading to slow synchronization and a less responsive network.
2.  **Storage Overhead:** Requiring every peer to store a copy of every blob from all other peers would lead to excessive storage consumption. This design allows peers to only store the blobs they explicitly need.
3.  **On-Demand Fetching:** The synchronized metadata includes a reference to the blob (like a path or content hash). A peer that needs the full blob can use this reference to request it directly from the origin peer or a future dedicated storage layer (like IPFS). This pattern of "synchronizing discovery, not data" is a standard practice for keeping distributed systems fast and scalable.


### KNIRVQL: The KNIRV Query Language

To facilitate easy interaction with the database, the system will feature a custom query language called **KNIRVQL**. It is designed to be a simple, human-readable language for performing CRUD operations and specialized queries on both `MEMORY` and `AUTH` entry types.

**Core Features of KNIRVQL:**

*   **Unified Interface:** A single language to manage authentication keys (`AUTH`) and complex data (`MEMORY`).
*   **Metadata Filtering:** Powerful filtering capabilities on the metadata facts table of `MEMORY` entries.
*   **Vector Search:** Native support for vector similarity searches on `MEMORY` entries.

**Example Queries:**

```knirvql
// Find the 10 most similar memory entries based on a vector, filtered by metadata
GET MEMORY WHERE metadata.source = "web-scrape" SIMILAR TO [0.45, 0.12, ...] LIMIT 10;

// Create or update an authentication key
SET AUTH key = "google_maps_api_key" value = "AIzaSy...";

// Retrieve an authentication key
GET AUTH WHERE key = "google_maps_api_key";

// Delete an entry by its ID
DELETE WHERE id = "entry-id-12345";
```


## Technology Stack

- **DHT/P2P Layer:** libp2p (production-grade P2P networking)
- **Conflict Resolution:** CRDT (Conflict-free Replicated Data Types) with vector clocks
- **Transport:** TCP, WebSocket, WebRTC (via libp2p)
- **Discovery:** mDNS (local), Bootstrap nodes (remote)
- **Encryption:** TLS 1.3 via libp2p noise protocol

## Core Components

### 1. Network Types and Interfaces

**File:** `packages/core/src/distributed/types.ts`

```typescript
import type { PeerId } from '@libp2p/interface-peer-id';
import type { Multiaddr } from '@multiformats/multiaddr';
import { Document } from '../types';

/**
 * Vector clock for tracking document versions across peers
 */
export interface VectorClock {
  [peerId: string]: number;
}

/**
 * Type of entry stored in the database.
 * MEMORY: For complex data with multi-file persistence (blob, vector, metadata).
 * AUTH: For simple key-value authentication data.
 */
export enum EntryType {
  MEMORY = 'MEMORY',
  AUTH = 'AUTH',
}

/**
 * Document with distributed metadata
 */
export interface DistributedDocument extends Document {
  _entryType: EntryType;
  _vector: VectorClock;
  _timestamp: number;
  _peerId: string;
  /** Optional stage marker. Use 'post-pending' to stage the document for posting to KNIRVGRAPH during next sync */
  _stage?: string;
  _deleted?: boolean;
}

/**
 * Operation types for CRDT
 */
export enum OperationType {
  INSERT = 'insert',
  UPDATE = 'update',
  DELETE = 'delete'
}

/**
 * CRDT operation for synchronization
 */
export interface CRDTOperation {
  id: string;
  type: OperationType;
  collection: string;
  documentId: string | number;
  data: Partial<DistributedDocument>;
  vector: VectorClock;
  timestamp: number;
  peerId: string;
}

/**
 * Network configuration
 *
 * New optional fields control posting/staging behavior:
 *  - defaultPostingNetwork: network name to post staged entries to (eg. "knirvgraph")
 *  - autoPostClassifications: EntryType[] which are auto-staged for posting (eg. Error, Context, Idea)
 *  - privateByDefault: when true, entries are private unless staged or configured
 */
export interface NetworkConfig {
  networkId: string;
  name: string;
  collections: Set<string>;
  bootstrapPeers: Multiaddr[];
  // Default posting target for staged entries e.g. 'knirvgraph'
  defaultPostingNetwork?: string;
  // Entry types that should be auto-staged for posting
  autoPostClassifications?: EntryType[];
  // Defaults to true — entries remain private unless staged/configured
  privateByDefault?: boolean;
  encryption: {
    enabled: boolean;
    sharedSecret?: string;
  };
  replication: {
    factor: number;
    strategy: 'full' | 'partial' | 'leader';
  };
  discovery: {
    mdns: boolean;
    bootstrap: boolean;
  };
} 

/**
 * Peer information
 */
export interface PeerInfo {
  peerId: string;
  addresses: Multiaddr[];
  protocols: string[];
  latency?: number;
  lastSeen: number;
  collections: string[];
}

/**
 * Sync state for a collection
 */
export interface SyncState {
  collection: string;
  networkId: string;
  localVector: VectorClock;
  lastSync: number;
  pendingOperations: CRDTOperation[];
  /** IDs of documents staged with `_stage === 'post-pending'` */
  stagedEntries?: string[];
  syncInProgress: boolean;
}

/**
 * Network statistics
 */
export interface NetworkStats {
  networkId: string;
  connectedPeers: number;
  totalPeers: number;
  collectionsShared: number;
  operationsSent: number;
  operationsReceived: number;
  bytesTransferred: number;
  averageLatency: number;
}

/**
 * Message types for peer communication
 */
export enum MessageType {
  SYNC_REQUEST = 'sync_request',
  SYNC_RESPONSE = 'sync_response',
  OPERATION = 'operation',
  HEARTBEAT = 'heartbeat',
  COLLECTION_ANNOUNCE = 'collection_announce',
  COLLECTION_REQUEST = 'collection_request'
}

/**
 * Protocol message
 */
export interface ProtocolMessage {
  type: MessageType;
  networkId: string;
  senderId: string;
  timestamp: number;
  payload: any;
}
```

### Post-Pending Staging

Only documents with `_stage === 'post-pending'` are synchronized with a chosen network. By default the system will auto-stage entries classified as **Error**, **Context**, and **Idea** to the `KNIRVGRAPH` posting network unless the `NetworkConfig` overrides this behavior. All other entries are **private by default** and will not be synchronized unless explicitly staged or configured to auto-post.

Behavior summary:

- **Only staged entries are synchronized** with a network; staging is the explicit signal to prepare a document for posting.
- `_stage === 'post-pending'` documents remain local and are **not** broadcast as regular CRDT operations.
- `SyncState.stagedEntries` stores document IDs for each collection/network.
- Use `NetworkConfig.defaultPostingNetwork`, `NetworkConfig.autoPostClassifications`, and `NetworkConfig.privateByDefault` to control defaults and override the standard behaviour.
- During the next sync, staged entries are converted to transactions for the configured posting network and submitted; on success the `_stage` is cleared and the ID removed from `stagedEntries`.

Example (TypeScript):

```ts
// Auto-stage on save when classification matches network config
async function saveDocument(db: Database, collection: string, doc: DistributedDocument) {
  const cfg = await db.loadNetworkConfig(collection);
  if (cfg?.autoPostClassifications?.includes(doc._payloadClassification as EntryType)) {
    doc._stage = 'post-pending';
    await db.save(collection, doc);
    const s = await db.loadSyncState(collection);
    s.stagedEntries = s.stagedEntries || [];
    s.stagedEntries.push(doc.id as string);
    await db.saveSyncState(collection, s);
    return;
  }
  // default save path (private by default unless configured)
  await db.save(collection, doc);
}

// Sync handler
async function handleSync(syncState: SyncState, db: Database, graphClient: KnirvClient) {
  const staged = syncState.stagedEntries || [];
  for (const docId of [...staged]) {
    const doc = await db.get(syncState.collection, docId);
    if (!doc || doc._stage !== 'post-pending') continue;

    const tx = buildKNIRVTransaction(doc);
    try {
      await graphClient.submitTransaction(tx);
      doc._stage = undefined;
      await db.save(syncState.collection, doc);
      syncState.stagedEntries = (syncState.stagedEntries || []).filter(id => id !== docId);
    } catch (err) {
      // keep for retry
    }
  }
  await db.saveSyncState(syncState.collection, syncState);
}
```

Note: Posting to KNIRVGRAPH is an orthogonal operation to the CRDT sync; CRDTs continue to reconcile document state across peers while `post-pending` marks an explicit intent to publish a document as a graph transaction.

### 2. Vector Clock Implementation

**File:** `packages/core/src/distributed/vector-clock.ts`

```typescript
import { VectorClock } from './types';

export class VectorClockManager {
  /**
   * Increment the clock for a specific peer
   */
  static increment(clock: VectorClock, peerId: string): VectorClock {
    return {
      ...clock,
      [peerId]: (clock[peerId] || 0) + 1
    };
  }

  /**
   * Merge two vector clocks (take maximum of each component)
   */
  static merge(clock1: VectorClock, clock2: VectorClock): VectorClock {
    const allPeers = new Set([...Object.keys(clock1), ...Object.keys(clock2)]);
    const merged: VectorClock = {};

    for (const peerId of allPeers) {
      merged[peerId] = Math.max(clock1[peerId] || 0, clock2[peerId] || 0);
    }

    return merged;
  }

  /**
   * Compare two vector clocks
   * Returns: 'before' | 'after' | 'concurrent' | 'equal'
   */
  static compare(clock1: VectorClock, clock2: VectorClock): 'before' | 'after' | 'concurrent' | 'equal' {
    const allPeers = new Set([...Object.keys(clock1), ...Object.keys(clock2)]);

    let hasGreater = false;
    let hasLess = false;

    for (const peerId of allPeers) {
      const val1 = clock1[peerId] || 0;
      const val2 = clock2[peerId] || 0;

      if (val1 > val2) hasGreater = true;
      if (val1 < val2) hasLess = true;
    }

    if (!hasGreater && !hasLess) return 'equal';
    if (hasGreater && !hasLess) return 'after';
    if (hasLess && !hasGreater) return 'before';
    return 'concurrent';
  }

  /**
   * Check if clock1 is causally before clock2
   */
  static happensBefore(clock1: VectorClock, clock2: VectorClock): boolean {
    const comparison = this.compare(clock1, clock2);
    return comparison === 'before' || comparison === 'equal';
  }

  /**
   * Create a new empty vector clock
   */
  static create(): VectorClock {
    return {};
  }

  /**
   * Clone a vector clock
   */
  static clone(clock: VectorClock): VectorClock {
    return { ...clock };
  }
}
```

### 3. CRDT Conflict Resolution

**File:** `packages/core/src/distributed/crdt-resolver.ts`

```typescript
import { DistributedDocument, CRDTOperation, OperationType, VectorClock } from './types';
import { VectorClockManager } from './vector-clock';
import { Document } from '../types';

export class CRDTResolver {
  /**
   * Resolve conflicts between two document versions
   * Uses Last-Write-Wins (LWW) with vector clock tie-breaking
   * Note: Documents with `_stage === 'post-pending'` represent local intent and are not
   * emitted as CRDT operations until they are posted as KNIRVGRAPH transactions during the sync process.
   */
  static resolveConflict(local: DistributedDocument, remote: DistributedDocument): DistributedDocument {
    // Handle deletion
    if (remote._deleted && !local._deleted) {
      const comparison = VectorClockManager.compare(local._vector, remote._vector);
      if (comparison === 'before' || comparison === 'concurrent') {
        return remote;
      }
      return local;
    }

    if (local._deleted && !remote._deleted) {
      const comparison = VectorClockManager.compare(remote._vector, local._vector);
      if (comparison === 'before' || comparison === 'concurrent') {
        return local;
      }
      return remote;
    }

    const comparison = VectorClockManager.compare(local._vector, remote._vector);

    switch (comparison) {
      case 'after':
        return local;
      case 'before':
        return remote;
      case 'equal':
        return local; // Already in sync
      case 'concurrent':
        // Use timestamp and peer ID for deterministic resolution
        if (local._timestamp > remote._timestamp) {
          return this.mergeDocuments(local, remote);
        } else if (local._timestamp < remote._timestamp) {
          return this.mergeDocuments(remote, local);
        } else {
          // Same timestamp - use peer ID for deterministic ordering
          return local._peerId > remote._peerId
            ? this.mergeDocuments(local, remote)
            : this.mergeDocuments(remote, local);
        }
    }
  }

  /**
   * Merge two documents (winner wins, but preserve concurrent field updates)
   */
  private static mergeDocuments(winner: DistributedDocument, loser: DistributedDocument): DistributedDocument {
    const merged: DistributedDocument = { ...winner };
    merged._vector = VectorClockManager.merge(winner._vector, loser._vector);

    // Merge fields that don't conflict
    for (const key in loser) {
      if (key.startsWith('_')) continue; // Skip metadata
      if (!(key in winner)) {
        merged[key] = loser[key];
      }
    }

    return merged;
  }

  /**
   * Apply an operation to a document
   */
  static applyOperation(doc: DistributedDocument | null, op: CRDTOperation): DistributedDocument | null {
    switch (op.type) {
      case OperationType.INSERT:
      case OperationType.UPDATE:
        if (!doc) {
          return {
            ...op.data as DistributedDocument,
            _vector: op.vector,
            _timestamp: op.timestamp,
            _peerId: op.peerId
          };
        }

        const comparison = VectorClockManager.compare(doc._vector, op.vector);
        if (comparison === 'before' || comparison === 'concurrent') {
          const updated = { ...doc, ...op.data };
          updated._vector = VectorClockManager.merge(doc._vector, op.vector);
          updated._timestamp = Math.max(doc._timestamp, op.timestamp);
          return updated;
        }
        return doc;

      case OperationType.DELETE:
        if (!doc) return null;

        const delComparison = VectorClockManager.compare(doc._vector, op.vector);
        if (delComparison === 'before' || delComparison === 'concurrent') {
          return {
            ...doc,
            _deleted: true,
            _vector: VectorClockManager.merge(doc._vector, op.vector),
            _timestamp: Math.max(doc._timestamp, op.timestamp)
          };
        }
        return doc;

      default:
        return doc;
    }
  }

  /**
   * Convert a regular document to a distributed document
   */
  static toDistributedDocument(doc: Document, peerId: string): DistributedDocument {
    const vector = VectorClockManager.create();
    vector[peerId] = 1;

    return {
      ...doc,
      _vector: vector,
      _timestamp: Date.now(),
      _peerId: peerId
    };
  }

  /**
   * Convert a distributed document back to a regular document
   */
  static toRegularDocument(doc: DistributedDocument): Document {
    const { _vector, _timestamp, _peerId, _deleted, ...regularDoc } = doc;
    return regularDoc;
  }
}
```

### 4. Network Manager

**File:** `packages/core/src/distributed/network-manager.ts`

```typescript
import { createLibp2p, Libp2p } from 'libp2p';
import { tcp } from '@libp2p/tcp';
import { noise } from '@chainsafe/libp2p-noise';
import { mplex } from '@libp2p/mplex';
import { bootstrap } from '@libp2p/bootstrap';
import { mdns } from '@libp2p/mdns';
import { kadDHT } from '@libp2p/kad-dht';
import type { PeerId } from '@libp2p/interface-peer-id';
import { pipe } from 'it-pipe';
import { pushable } from 'it-pushable';
import {
  NetworkConfig,
  PeerInfo,
  NetworkStats,
  ProtocolMessage,
  MessageType
} from './types';
import { EventEmitter } from 'events';

export class NetworkManager extends EventEmitter {
  private networks: Map<string, NetworkConfig> = new Map();
  private node: Libp2p | null = null;
  private peers: Map<string, PeerInfo> = new Map();
  private stats: Map<string, NetworkStats> = new Map();
  private messageHandlers: Map<MessageType, ((msg: ProtocolMessage) => Promise<void>)[]> = new Map();
  private initialized = false;
  private peerId: string = '';

  constructor() {
    super();
  }

  /**
   * Initialize the P2P node
   */
  async initialize(): Promise<void> {
    if (this.initialized) return;

    this.node = await createLibp2p({
      addresses: {
        listen: ['/ip4/0.0.0.0/tcp/0']
      },
      transports: [tcp()],
      connectionEncryption: [noise()],
      streamMuxers: [mplex()],
      peerDiscovery: [
        mdns({
          interval: 1000
        })
      ],
      services: {
        dht: kadDHT({
          clientMode: false
        })
      }
    });

    await this.node.start();
    this.peerId = this.node.peerId.toString();
    this.initialized = true;

    // Set up connection handlers
    this.node.addEventListener('peer:discovery', (evt) => {
      this.handlePeerDiscovery(evt.detail);
    });

    this.node.addEventListener('peer:connect', (evt) => {
      this.handlePeerConnect(evt.detail);
    });

    this.node.addEventListener('peer:disconnect', (evt) => {
      this.handlePeerDisconnect(evt.detail);
    });

    this.emit('initialized', { peerId: this.peerId });
  }

  /**
   * Create a new network
   */
  async createNetwork(config: Omit<NetworkConfig, 'collections'>): Promise<string> {
    if (!this.initialized) {
      await this.initialize();
    }

    const networkId = config.networkId;

    const networkConfig: NetworkConfig = {
      ...config,
      collections: new Set()
    };

    this.networks.set(networkId, networkConfig);

    // Initialize stats
    this.stats.set(networkId, {
      networkId,
      connectedPeers: 0,
      totalPeers: 0,
      collectionsShared: 0,
      operationsSent: 0,
      operationsReceived: 0,
      bytesTransferred: 0,
      averageLatency: 0
    });

    // Register protocol handler for this network
    const protocolId = `/nebulusdb/${networkId}/1.0.0`;
    await this.node!.handle(protocolId, async ({ stream }) => {
      await this.handleIncomingStream(stream, networkId);
    });

    this.emit('network:created', { networkId });
    return networkId;
  }

  /**
   * Join an existing network
   */
  async joinNetwork(networkId: string, bootstrapPeers: string[]): Promise<void> {
    if (!this.initialized) {
      await this.initialize();
    }

    const config: NetworkConfig = {
      networkId,
      name: `Network ${networkId}`,
      collections: new Set(),
      bootstrapPeers: bootstrapPeers.map(addr => {
        const { multiaddr } = require('@multiformats/multiaddr');
        return multiaddr(addr);
      }),
      encryption: { enabled: true },
      replication: { factor: 3, strategy: 'full' },
      discovery: { mdns: true, bootstrap: true }
    };

    this.networks.set(networkId, config);

    // Connect to bootstrap peers
    for (const addr of config.bootstrapPeers) {
      try {
        await this.node!.dial(addr);
      } catch (err) {
        console.warn(`Failed to connect to bootstrap peer ${addr}:`, err);
      }
    }

    this.emit('network:joined', { networkId });
  }

  /**
   * Leave a network
   */
  async leaveNetwork(networkId: string): Promise<void> {
    const network = this.networks.get(networkId);
    if (!network) return;

    // Unregister protocol handler
    const protocolId = `/nebulusdb/${networkId}/1.0.0`;
    this.node!.unhandle(protocolId);

    // Clear collections
    network.collections.clear();

    // Remove network
    this.networks.delete(networkId);
    this.stats.delete(networkId);

    this.emit('network:left', { networkId });
  }

  /**
   * Add a collection to a network
   */
  async addCollectionToNetwork(networkId: string, collectionName: string): Promise<void> {
    const network = this.networks.get(networkId);
    if (!network) {
      throw new Error(`Network ${networkId} not found`);
    }

    network.collections.add(collectionName);

    const stats = this.stats.get(networkId)!;
    stats.collectionsShared = network.collections.size;

    // Announce collection to peers
    await this.broadcastMessage(networkId, {
      type: MessageType.COLLECTION_ANNOUNCE,
      networkId,
      senderId: this.peerId,
      timestamp: Date.now(),
      payload: { collection: collectionName }
    });

    this.emit('collection:added', { networkId, collectionName });
  }

  /**
   * Remove a collection from a network
   */
  async removeCollectionFromNetwork(networkId: string, collectionName: string): Promise<void> {
    const network = this.networks.get(networkId);
    if (!network) return;

    network.collections.delete(collectionName);

    const stats = this.stats.get(networkId)!;
    stats.collectionsShared = network.collections.size;

    this.emit('collection:removed', { networkId, collectionName });
  }

  /**
   * Get collections in a network
   */
  getNetworkCollections(networkId: string): string[] {
    const network = this.networks.get(networkId);
    return network ? Array.from(network.collections) : [];
  }

  /**
   * Send a message to all peers in a network
   */
  async broadcastMessage(networkId: string, message: ProtocolMessage): Promise<void> {
    const network = this.networks.get(networkId);
    if (!network) return;

    const protocolId = `/nebulusdb/${networkId}/1.0.0`;
    const connections = this.node!.getConnections();

    const stats = this.stats.get(networkId)!;

    for (const connection of connections) {
      try {
        const stream = await connection.newStream(protocolId);
        const messageBytes = Buffer.from(JSON.stringify(message));

        await pipe(
          [messageBytes],
          stream
        );

        stats.operationsSent++;
        stats.bytesTransferred += messageBytes.length;
      } catch (err) {
        console.warn(`Failed to send message to peer ${connection.remotePeer}:`, err);
      }
    }
  }

  /**
   * Send a message to a specific peer
   */
  async sendToPeer(peerId: string, networkId: string, message: ProtocolMessage): Promise<void> {
    const protocolId = `/nebulusdb/${networkId}/1.0.0`;
    const { peerIdFromString } = await import('@libp2p/peer-id');
    const targetPeerId = peerIdFromString(peerId);

    try {
      const stream = await this.node!.dialProtocol(targetPeerId, protocolId);
      const messageBytes = Buffer.from(JSON.stringify(message));

      await pipe(
        [messageBytes],
        stream
      );

      const stats = this.stats.get(networkId)!;
      stats.operationsSent++;
      stats.bytesTransferred += messageBytes.length;
    } catch (err) {
      console.error(`Failed to send message to peer ${peerId}:`, err);
      throw err;
    }
  }

  /**
   * Register a message handler
   */
  onMessage(type: MessageType, handler: (msg: ProtocolMessage) => Promise<void>): void {
    if (!this.messageHandlers.has(type)) {
      this.messageHandlers.set(type, []);
    }
    this.messageHandlers.get(type)!.push(handler);
  }

  /**
   * Get network statistics
   */
  getNetworkStats(networkId: string): NetworkStats | null {
    return this.stats.get(networkId) || null;
  }

  /**
   * Get all networks
   */
  getNetworks(): NetworkConfig[] {
    return Array.from(this.networks.values());
  }

  /**
   * Get connected peers
   */
  getConnectedPeers(): PeerInfo[] {
    return Array.from(this.peers.values());
  }

  /**
   * Get peer ID
   */
  getPeerId(): string {
    return this.peerId;
  }

  /**
   * Shutdown the network manager
   */
  async shutdown(): Promise<void> {
    if (this.node) {
      await this.node.stop();
      this.initialized = false;
    }

    this.networks.clear();
    this.peers.clear();
    this.stats.clear();
    this.messageHandlers.clear();
  }

  // Private methods

  private async handleIncomingStream(stream: any, networkId: string): Promise<void> {
    const stats = this.stats.get(networkId)!;

    try {
      await pipe(
        stream,
        async (source: any) => {
          for await (const msg of source) {
            const message: ProtocolMessage = JSON.parse(msg.toString());
            stats.operationsReceived++;
            stats.bytesTransferred += msg.length;

            await this.handleMessage(message);
          }
        }
      );
    } catch (err) {
      console.error('Error handling incoming stream:', err);
    }
  }

  private async handleMessage(message: ProtocolMessage): Promise<void> {
    const handlers = this.messageHandlers.get(message.type);
    if (handlers) {
      for (const handler of handlers) {
        try {
          await handler(message);
        } catch (err) {
          console.error(`Error in message handler for type ${message.type}:`, err);
        }
      }
    }

    this.emit('message:received', message);
  }

  private handlePeerDiscovery(peerId: PeerId): void {
    const peerIdStr = peerId.toString();

    if (!this.peers.has(peerIdStr)) {
      this.peers.set(peerIdStr, {
        peerId: peerIdStr,
        addresses: [],
        protocols: [],
        lastSeen: Date.now(),
        collections: []
      });
    }

    this.emit('peer:discovered', { peerId: peerIdStr });
  }

  private handlePeerConnect(peerId: PeerId): void {
    const peerIdStr = peerId.toString();

    const peerInfo = this.peers.get(peerIdStr);
    if (peerInfo) {
      peerInfo.lastSeen = Date.now();
    }

    // Update stats for all networks
    for (const [networkId, stats] of this.stats.entries()) {
      stats.connectedPeers = this.node!.getConnections().length;
      stats.totalPeers = this.peers.size;
    }

    this.emit('peer:connected', { peerId: peerIdStr });
  }

  private handlePeerDisconnect(peerId: PeerId): void {
    const peerIdStr = peerId.toString();

    // Update stats for all networks
    for (const [networkId, stats] of this.stats.entries()) {
      stats.connectedPeers = this.node!.getConnections().length;
    }

    this.emit('peer:disconnected', { peerId: peerIdStr });
  }
}
```

### 5. Distributed Collection

**File:** `packages/core/src/distributed/distributed-collection.ts`

```typescript
import { Collection } from '../collection';
import {
  Document,
  Query,
  UpdateOperation,
  CollectionOptions
} from '../types';
import {
  DistributedDocument,
  CRDTOperation,
  OperationType,
  SyncState,
  MessageType,
  ProtocolMessage
} from './types';
import { NetworkManager } from './network-manager';
import { CRDTResolver } from './crdt-resolver';
import { VectorClockManager } from './vector-clock';

export class DistributedCollection extends Collection {
  private networkManager: NetworkManager;
  private networkId: string | null = null;
  private syncStates: Map<string, SyncState> = new Map();
  private operationLog: CRDTOperation[] = [];
  private maxLogSize = 10000;

  constructor(
    name: string,
    networkManager: NetworkManager,
    initialDocs: Document[] = [],
    options: CollectionOptions = {},
    plugins: any[] = []
  ) {
    super(name, initialDocs, options, plugins);
    this.networkManager = networkManager;

    // Register message handlers
    this.setupMessageHandlers();
  }

  /**
   * Attach this collection to a network
   */
  async attachToNetwork(networkId: string): Promise<void> {
    if (this.networkId) {
      throw new Error(`Collection ${this.name} is already attached to network ${this.networkId}`);
    }

    this.networkId = networkId;
    await this.networkManager.addCollectionToNetwork(networkId, this.name);

    // Initialize sync state
    const syncState: SyncState = {
      collection: this.name,
      networkId,
      localVector: VectorClockManager.create(),
      lastSync: Date.now(),
      pendingOperations: [],
      syncInProgress: false
    };
    this.syncStates.set(networkId, syncState);

    // Request initial sync
    await this.requestSync();
  }

  /**
   * Detach this collection from its network
   */
  async detachFromNetwork(): Promise<void> {
    if (!this.networkId) return;

    await this.networkManager.removeCollectionFromNetwork(this.networkId, this.name);
    this.syncStates.delete(this.networkId);
    this.networkId = null;
  }

  /**
   * Insert with distributed synchronization
   */
  async insert(doc: Omit<Document, 'id'> & { id?: string; _entryType?: EntryType; payload?: any }): Promise<Document> {
    if (!doc._entryType || !Object.values(EntryType).includes(doc._entryType)) {
        throw new Error("Document must have a valid '_entryType' ('MEMORY' or 'AUTH')");
    }

    if (doc._entryType === EntryType.MEMORY && doc.payload?.blob) {
        // For MEMORY entries, we separate the blob for local storage.
        const blobData = doc.payload.blob;
        delete doc.payload.blob;

        // 1. Save blobData to a local file in the app data directory.
        //    This is a placeholder for the actual file I/O logic.
        //    const blobPath = await saveBlobToLocalStorage(doc.id, blobData);

        // 2. The payload is modified for synchronization. The blob data is
        //    removed, and a reference is stored instead. The metadata and
        //    vector remain in the payload to be synchronized.
        // doc.payload.blobRef = blobPath;
    }
    
    // The local collection handles persistence. The underlying adapter is responsible
    // for the actual file I/O for blobs.
    const inserted = await super.insert(doc);

    if (this.networkId) {
      const distributedDoc = this.toDistributed(inserted);
      (distributedDoc as any)._entryType = doc._entryType;

      await this.broadcastOperation({
        id: `${this.networkManager.getPeerId()}-${Date.now()}-${Math.random()}`,
        type: OperationType.INSERT,
        collection: this.name,
        documentId: inserted.id,
        data: distributedDoc,
        vector: this.getCurrentVector(),
        timestamp: Date.now(),
        peerId: this.networkManager.getPeerId()
      });
    }

    return inserted;
  }

  /**
   * Update with distributed synchronization
   */
  async update(query: Query, update: UpdateOperation): Promise<number> {
    const affected = await super.update(query, update);

    if (this.networkId && affected > 0) {
      const docs = await super.find(query);

      for (const doc of docs) {
        await this.broadcastOperation({
          id: `${this.networkManager.getPeerId()}-${Date.now()}-${Math.random()}`,
          type: OperationType.UPDATE,
          collection: this.name,
          documentId: doc.id,
          data: this.toDistributed(doc),
          vector: this.getCurrentVector(),
          timestamp: Date.now(),
          peerId: this.networkManager.getPeerId()
        });
      }
    }

    return affected;
  }

  /**
   * Delete with distributed synchronization
   */
  async delete(query: Query): Promise<number> {
    const docsToDelete = await super.find(query);
    const deleted = await super.delete(query);

    if (this.networkId && deleted > 0) {
      for (const doc of docsToDelete) {
        await this.broadcastOperation({
          id: `${this.networkManager.getPeerId()}-${Date.now()}-${Math.random()}`,
          type: OperationType.DELETE,
          collection: this.name,
          documentId: doc.id,
          data: { ...this.toDistributed(doc), _deleted: true },
          vector: this.getCurrentVector(),
          timestamp: Date.now(),
          peerId: this.networkManager.getPeerId()
        });
      }
    }

    return deleted;
  }

  /**
   * Get sync state for the current network
   */
  getSyncState(): SyncState | null {
    if (!this.networkId) return null;
    return this.syncStates.get(this.networkId) || null;
  }

  /**
   * Force synchronization with peers
   */
  async forceSync(): Promise<void> {
    if (!this.networkId) return;
    await this.requestSync();
  }

  // Private methods

  private setupMessageHandlers(): void {
    this.networkManager.onMessage(MessageType.OPERATION, async (msg) => {
      if (msg.payload.collection === this.name) {
        await this.handleRemoteOperation(msg.payload.operation);
      }
    });

    this.networkManager.onMessage(MessageType.SYNC_REQUEST, async (msg) => {
      if (msg.payload.collection === this.name) {
        await this.handleSyncRequest(msg);
      }
    });

    this.networkManager.onMessage(MessageType.SYNC_RESPONSE, async (msg) => {
      if (msg.payload.collection === this.name) {
        await this.handleSyncResponse(msg);
      }
    });
  }

  private async broadcastOperation(operation: CRDTOperation): Promise<void> {
    if (!this.networkId) return;

    // Add to operation log
    this.operationLog.push(operation);
    this.pruneOperationLog();

    // Update local vector clock
    const syncState = this.syncStates.get(this.networkId)!;
    syncState.localVector = VectorClockManager.increment(
      syncState.localVector,
      this.networkManager.getPeerId()
    );

    // Broadcast to network
    await this.networkManager.broadcastMessage(this.networkId, {
      type: MessageType.OPERATION,
      networkId: this.networkId,
      senderId: this.networkManager.getPeerId(),
      timestamp: Date.now(),
      payload: { collection: this.name, operation }
    });
  }

  private async handleRemoteOperation(operation: CRDTOperation): Promise<void> {
    // Find existing document
    const docs = await super.find({ id: operation.documentId });
    const existingDoc = docs[0] || null;

    // Apply CRDT operation
    const distributedDoc = existingDoc ? this.toDistributed(existingDoc) : null;
    const result = CRDTResolver.applyOperation(distributedDoc, operation);

    if (result === null) {
      // Document was deleted
      if (existingDoc) {
        await super.delete({ id: operation.documentId });
      }
    } else if (result._deleted) {
      // Mark as deleted
      await super.delete({ id: operation.documentId });
    } else {
      // Update or insert document
      const regularDoc = CRDTResolver.toRegularDocument(result);

      if (existingDoc) {
        await super.update({ id: operation.documentId }, { $set: regularDoc });
      } else {
        await super.insert(regularDoc);
      }
    }

    // Update vector clock
    if (this.networkId) {
      const syncState = this.syncStates.get(this.networkId)!;
      syncState.localVector = VectorClockManager.merge(
        syncState.localVector,
        operation.vector
      );
    }
  }

  private async requestSync(): Promise<void> {
    if (!this.networkId) return;

    const syncState = this.syncStates.get(this.networkId)!;
    if (syncState.syncInProgress) return;

    syncState.syncInProgress = true;

    await this.networkManager.broadcastMessage(this.networkId, {
      type: MessageType.SYNC_REQUEST,
      networkId: this.networkId,
      senderId: this.networkManager.getPeerId(),
      timestamp: Date.now(),
      payload: {
        collection: this.name,
        vector: syncState.localVector
      }
    });

    // Timeout after 10 seconds
    setTimeout(() => {
      syncState.syncInProgress = false;
    }, 10000);
  }

  private async handleSyncRequest(msg: ProtocolMessage): Promise<void> {
    if (!this.networkId) return;

    const remoteVector = msg.payload.vector;
    const localVector = this.syncStates.get(this.networkId)!.localVector;

    // Find operations that remote peer doesn't have
    const missingOps = this.operationLog.filter(op => {
      const remoteClock = remoteVector[op.peerId] || 0;
      const opClock = op.vector[op.peerId] || 0;
      return opClock > remoteClock;
    });

    // Send response
    await this.networkManager.sendToPeer(msg.senderId, this.networkId, {
      type: MessageType.SYNC_RESPONSE,
      networkId: this.networkId,
      senderId: this.networkManager.getPeerId(),
      timestamp: Date.now(),
      payload: {
        collection: this.name,
        operations: missingOps,
        vector: localVector
      }
    });
  }

  private async handleSyncResponse(msg: ProtocolMessage): Promise<void> {
    if (!this.networkId) return;

    const operations: CRDTOperation[] = msg.payload.operations;

    // Apply operations in order
    for (const op of operations) {
      await this.handleRemoteOperation(op);
    }

    const syncState = this.syncStates.get(this.networkId)!;
    syncState.syncInProgress = false;
    syncState.lastSync = Date.now();
  }

  private toDistributed(doc: Document): DistributedDocument {
    return CRDTResolver.toDistributedDocument(doc, this.networkManager.getPeerId());
  }

  private getCurrentVector(): import('./types').VectorClock {
    if (!this.networkId) return VectorClockManager.create();

    const syncState = this.syncStates.get(this.networkId);
    return syncState ? syncState.localVector : VectorClockManager.create();
  }

  private pruneOperationLog(): void {
    if (this.operationLog.length > this.maxLogSize) {
      // Keep only the most recent operations
      this.operationLog = this.operationLog.slice(-this.maxLogSize);
    }
  }
}
```

### 6. Distributed Database

**File:** `packages/core/src/distributed/distributed-database.ts`

```typescript
import { Database } from '../db';
import { DbOptions, CollectionOptions, ICollection } from '../types';
import { NetworkManager } from './network-manager';
import { DistributedCollection } from './distributed-collection';

export interface DistributedDbOptions extends DbOptions {
  distributed: {
    enabled: boolean;
    networkId?: string;
    bootstrapPeers?: string[];
  };
}

export class DistributedDatabase extends Database {
  private networkManager: NetworkManager;
  private distributedEnabled: boolean;

  constructor(options: DistributedDbOptions) {
    super(options);

    this.distributedEnabled = options.distributed.enabled;
    this.networkManager = new NetworkManager();

    if (this.distributedEnabled) {
      this.initializeDistributed(options.distributed);
    }
  }

  /**
   * Create or get a collection with distributed support
   */
  collection(name: string, options: CollectionOptions = {}): ICollection {
    if (this.collections.has(name)) {
      return this.collections.get(name)!;
    }

    let collection: ICollection;

    if (this.distributedEnabled) {
      collection = new DistributedCollection(
        name,
        this.networkManager,
        [],
        options,
        this.plugins
      );
    } else {
      collection = super.collection(name, options);
    }

    this.collections.set(name, collection);
    return collection;
  }

  /**
   * Create a new network
   */
  async createNetwork(config: {
    networkId: string;
    name: string;
    bootstrapPeers?: string[];
  }): Promise<string> {
    const { multiaddr } = await import('@multiformats/multiaddr');

    return await this.networkManager.createNetwork({
      networkId: config.networkId,
      name: config.name,
      bootstrapPeers: (config.bootstrapPeers || []).map(addr => multiaddr(addr)),
      encryption: { enabled: true },
      replication: { factor: 3, strategy: 'full' },
      discovery: { mdns: true, bootstrap: true }
    });
  }

  /**
   * Join an existing network
   */
  async joinNetwork(networkId: string, bootstrapPeers: string[]): Promise<void> {
    await this.networkManager.joinNetwork(networkId, bootstrapPeers);
  }

  /**
   * Leave a network
   */
  async leaveNetwork(networkId: string): Promise<void> {
    await this.networkManager.leaveNetwork(networkId);
  }

  /**
   * Add a collection to a network
   */
  async addCollectionToNetwork(networkId: string, collectionName: string): Promise<void> {
    const collection = this.collections.get(collectionName);

    if (!collection) {
      throw new Error(`Collection ${collectionName} not found`);
    }

    if (collection instanceof DistributedCollection) {
      await collection.attachToNetwork(networkId);
    } else {
      throw new Error(`Collection ${collectionName} is not a distributed collection`);
    }
  }

  /**
   * Remove a collection from a network
   */
  async removeCollectionFromNetwork(collectionName: string): Promise<void> {
    const collection = this.collections.get(collectionName);

    if (collection && collection instanceof DistributedCollection) {
      await collection.detachFromNetwork();
    }
  }

  /**
   * Get network manager
   */
  getNetworkManager(): NetworkManager {
    return this.networkManager;
  }

  /**
   * Shutdown the distributed database
   */
  async shutdown(): Promise<void> {
    await this.networkManager.shutdown();
  }

  // Private methods

  private async initializeDistributed(config: {
    networkId?: string;
    bootstrapPeers?: string[];
  }): Promise<void> {
    await this.networkManager.initialize();

    if (config.networkId) {
      if (config.bootstrapPeers && config.bootstrapPeers.length > 0) {
        await this.networkManager.joinNetwork(config.networkId, config.bootstrapPeers);
      } else {
        await this.networkManager.createNetwork({
          networkId: config.networkId,
          name: `Network ${config.networkId}`,
          bootstrapPeers: [],
          encryption: { enabled: true },
          replication: { factor: 3, strategy: 'full' },
          discovery: { mdns: true, bootstrap: true }
        });
      }
    }
  }
}

/**
 * Create a distributed database
 */
export function createDistributedDb(options: DistributedDbOptions): DistributedDatabase {
  return new DistributedDatabase(options);
}
```

## Testing Implementation

### Test Suite 1: Vector Clock Tests

**File:** `packages/core/src/distributed/tests/vector-clock.test.ts`

```typescript
import { describe, it, expect } from 'vitest';
import { VectorClockManager } from '../vector-clock';

describe('VectorClockManager', () => {
  it('should create an empty vector clock', () => {
    const clock = VectorClockManager.create();
    expect(clock).toEqual({});
  });

  it('should increment a peer clock', () => {
    let clock = VectorClockManager.create();
    clock = VectorClockManager.increment(clock, 'peer1');

    expect(clock).toEqual({ peer1: 1 });

    clock = VectorClockManager.increment(clock, 'peer1');
    expect(clock).toEqual({ peer1: 2 });
  });

  it('should merge two vector clocks', () => {
    const clock1 = { peer1: 3, peer2: 1 };
    const clock2 = { peer1: 2, peer2: 4, peer3: 1 };

    const merged = VectorClockManager.merge(clock1, clock2);

    expect(merged).toEqual({ peer1: 3, peer2: 4, peer3: 1 });
  });

  it('should compare equal clocks', () => {
    const clock1 = { peer1: 1, peer2: 2 };
    const clock2 = { peer1: 1, peer2: 2 };

    const result = VectorClockManager.compare(clock1, clock2);
    expect(result).toBe('equal');
  });

  it('should detect before relationship', () => {
    const clock1 = { peer1: 1, peer2: 1 };
    const clock2 = { peer1: 2, peer2: 2 };

    const result = VectorClockManager.compare(clock1, clock2);
    expect(result).toBe('before');
  });

  it('should detect after relationship', () => {
    const clock1 = { peer1: 3, peer2: 3 };
    const clock2 = { peer1: 2, peer2: 2 };

    const result = VectorClockManager.compare(clock1, clock2);
    expect(result).toBe('after');
  });

  it('should detect concurrent updates', () => {
    const clock1 = { peer1: 2, peer2: 1 };
    const clock2 = { peer1: 1, peer2: 2 };

    const result = VectorClockManager.compare(clock1, clock2);
    expect(result).toBe('concurrent');
  });

  it('should check happens-before relationship', () => {
    const clock1 = { peer1: 1, peer2: 1 };
    const clock2 = { peer1: 2, peer2: 2 };

    expect(VectorClockManager.happensBefore(clock1, clock2)).toBe(true);
    expect(VectorClockManager.happensBefore(clock2, clock1)).toBe(false);
  });

  it('should clone a vector clock', () => {
    const original = { peer1: 5, peer2: 3 };
    const cloned = VectorClockManager.clone(original);

    expect(cloned).toEqual(original);
    expect(cloned).not.toBe(original);

    cloned.peer1 = 10;
    expect(original.peer1).toBe(5);
  });
});
```

### Test Suite 2: CRDT Resolver Tests

**File:** `packages/core/src/distributed/tests/crdt-resolver.test.ts`

```typescript
import { describe, it, expect } from 'vitest';
import { CRDTResolver } from '../crdt-resolver';
import { DistributedDocument, CRDTOperation, OperationType } from '../types';
import { VectorClockManager } from '../vector-clock';

describe('CRDTResolver', () => {
  it('should convert document to distributed document', () => {
    const doc = { id: '1', name: 'Test', age: 30 };
    const peerId = 'peer1';

    const distDoc = CRDTResolver.toDistributedDocument(doc, peerId);

    expect(distDoc.id).toBe('1');
    expect(distDoc.name).toBe('Test');
    expect(distDoc.age).toBe(30);
    expect(distDoc._peerId).toBe(peerId);
    expect(distDoc._vector).toEqual({ peer1: 1 });
    expect(distDoc._timestamp).toBeGreaterThan(0);
  });

  it('should convert distributed document to regular document', () => {
    const distDoc: DistributedDocument = {
      id: '1',
      name: 'Test',
      _vector: { peer1: 5 },
      _timestamp: Date.now(),
      _peerId: 'peer1'
    };

    const doc = CRDTResolver.toRegularDocument(distDoc);

    expect(doc.id).toBe('1');
    expect(doc.name).toBe('Test');
    expect('_vector' in doc).toBe(false);
    expect('_timestamp' in doc).toBe(false);
    expect('_peerId' in doc).toBe(false);
  });

  it('should resolve conflict with clear winner (after)', () => {
    const local: DistributedDocument = {
      id: '1',
      name: 'Local',
      _vector: { peer1: 2 },
      _timestamp: Date.now(),
      _peerId: 'peer1'
    };

    const remote: DistributedDocument = {
      id: '1',
      name: 'Remote',
      _vector: { peer1: 1 },
      _timestamp: Date.now() - 1000,
      _peerId: 'peer2'
    };

    const result = CRDTResolver.resolveConflict(local, remote);
    expect(result.name).toBe('Local');
  });

  it('should resolve conflict with clear winner (before)', () => {
    const local: DistributedDocument = {
      id: '1',
      name: 'Local',
      _vector: { peer1: 1 },
      _timestamp: Date.now() - 1000,
      _peerId: 'peer1'
    };

    const remote: DistributedDocument = {
      id: '1',
      name: 'Remote',
      _vector: { peer1: 2 },
      _timestamp: Date.now(),
      _peerId: 'peer2'
    };

    const result = CRDTResolver.resolveConflict(local, remote);
    expect(result.name).toBe('Remote');
  });

  it('should resolve concurrent conflicts using timestamp', () => {
    const timestamp1 = Date.now();
    const timestamp2 = timestamp1 + 1000;

    const local: DistributedDocument = {
      id: '1',
      name: 'Local',
      _vector: { peer1: 1 },
      _timestamp: timestamp2,
      _peerId: 'peer1'
    };

    const remote: DistributedDocument = {
      id: '1',
      name: 'Remote',
      _vector: { peer2: 1 },
      _timestamp: timestamp1,
      _peerId: 'peer2'
    };

    const result = CRDTResolver.resolveConflict(local, remote);
    expect(result.name).toBe('Local');
    expect(result._vector).toEqual({ peer1: 1, peer2: 1 });
  });

  it('should apply insert operation', () => {
    const operation: CRDTOperation = {
      id: 'op1',
      type: OperationType.INSERT,
      collection: 'users',
      documentId: '1',
      data: {
        id: '1',
        name: 'New User',
        _vector: { peer1: 1 },
        _timestamp: Date.now(),
        _peerId: 'peer1'
      },
      vector: { peer1: 1 },
      timestamp: Date.now(),
      peerId: 'peer1'
    };

    const result = CRDTResolver.applyOperation(null, operation);

    expect(result).not.toBeNull();
    expect(result!.name).toBe('New User');
  });

  it('should apply update operation', () => {
    const existing: DistributedDocument = {
      id: '1',
      name: 'Old Name',
      age: 25,
      _vector: { peer1: 1 },
      _timestamp: Date.now() - 1000,
      _peerId: 'peer1'
    };

    const operation: CRDTOperation = {
      id: 'op2',
      type: OperationType.UPDATE,
      collection: 'users',
      documentId: '1',
      data: {
        id: '1',
        name: 'New Name',
        _vector: { peer1: 2 },
        _timestamp: Date.now(),
        _peerId: 'peer1'
      },
      vector: { peer1: 2 },
      timestamp: Date.now(),
      peerId: 'peer1'
    };

    const result = CRDTResolver.applyOperation(existing, operation);

    expect(result).not.toBeNull();
    expect(result!.name).toBe('New Name');
    expect(result!.age).toBe(25);
  });

  it('should apply delete operation', () => {
    const existing: DistributedDocument = {
      id: '1',
      name: 'To Delete',
      _vector: { peer1: 1 },
      _timestamp: Date.now() - 1000,
      _peerId: 'peer1'
    };

    const operation: CRDTOperation = {
      id: 'op3',
      type: OperationType.DELETE,
      collection: 'users',
      documentId: '1',
      data: {
        id: '1',
        _deleted: true,
        _vector: { peer1: 2 },
        _timestamp: Date.now(),
        _peerId: 'peer1'
      },
      vector: { peer1: 2 },
      timestamp: Date.now(),
      peerId: 'peer1'
    };

    const result = CRDTResolver.applyOperation(existing, operation);

    expect(result).not.toBeNull();
    expect(result!._deleted).toBe(true);
  });

  it('should ignore stale operations', () => {
    const existing: DistributedDocument = {
      id: '1',
      name: 'Current',
      _vector: { peer1: 5 },
      _timestamp: Date.now(),
      _peerId: 'peer1'
    };

    const operation: CRDTOperation = {
      id: 'op4',
      type: OperationType.UPDATE,
      collection: 'users',
      documentId: '1',
      data: {
        id: '1',
        name: 'Stale',
        _vector: { peer1: 3 },
        _timestamp: Date.now() - 5000,
        _peerId: 'peer1'
      },
      vector: { peer1: 3 },
      timestamp: Date.now() - 5000,
      peerId: 'peer1'
    };

    const result = CRDTResolver.applyOperation(existing, operation);

    expect(result!.name).toBe('Current');
  });

  it('should handle deleted document conflicts', () => {
    const local: DistributedDocument = {
      id: '1',
      name: 'Active',
      _vector: { peer1: 1 },
      _timestamp: Date.now() - 1000,
      _peerId: 'peer1'
    };

    const remote: DistributedDocument = {
      id: '1',
      _deleted: true,
      _vector: { peer2: 1 },
      _timestamp: Date.now(),
      _peerId: 'peer2'
    };

    const result = CRDTResolver.resolveConflict(local, remote);
    expect(result._deleted).toBe(true);
  });
});
```

### Test Suite 3: Network Manager Tests

**File:** `packages/core/src/distributed/tests/network-manager.test.ts`

```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { NetworkManager } from '../network-manager';
import { MessageType } from '../types';

describe('NetworkManager', () => {
  let manager1: NetworkManager;
  let manager2: NetworkManager;

  beforeEach(async () => {
    manager1 = new NetworkManager();
    manager2 = new NetworkManager();
  });

  afterEach(async () => {
    await manager1.shutdown();
    await manager2.shutdown();
  });

  it('should initialize network manager', async () => {
    await manager1.initialize();

    const peerId = manager1.getPeerId();
    expect(peerId).toBeTruthy();
    expect(peerId.length).toBeGreaterThan(0);
  });

  it('should create a new network', async () => {
    await manager1.initialize();

    const networkId = await manager1.createNetwork({
      networkId: 'test-network',
      name: 'Test Network',
      bootstrapPeers: [],
      encryption: { enabled: true },
      replication: { factor: 3, strategy: 'full' },
      discovery: { mdns: true, bootstrap: false }
    });

    expect(networkId).toBe('test-network');

    const networks = manager1.getNetworks();
    expect(networks).toHaveLength(1);
    expect(networks[0].networkId).toBe('test-network');
  });

  it('should add collection to network', async () => {
    await manager1.initialize();
    const networkId = await manager1.createNetwork({
      networkId: 'test-network',
      name: 'Test Network',
      bootstrapPeers: [],
      encryption: { enabled: true },
      replication: { factor: 3, strategy: 'full' },
      discovery: { mdns: true, bootstrap: false }
    });

    await manager1.addCollectionToNetwork(networkId, 'users');

    const collections = manager1.getNetworkCollections(networkId);
    expect(collections).toContain('users');
  });

  it('should remove collection from network', async () => {
    await manager1.initialize();
    const networkId = await manager1.createNetwork({
      networkId: 'test-network',
      name: 'Test Network',
      bootstrapPeers: [],
      encryption: { enabled: true },
      replication: { factor: 3, strategy: 'full' },
      discovery: { mdns: true, bootstrap: false }
    });

    await manager1.addCollectionToNetwork(networkId, 'users');
    await manager1.removeCollectionFromNetwork(networkId, 'users');

    const collections = manager1.getNetworkCollections(networkId);
    expect(collections).not.toContain('users');
  });

  it('should leave a network', async () => {
    await manager1.initialize();
    const networkId = await manager1.createNetwork({
      networkId: 'test-network',
      name: 'Test Network',
      bootstrapPeers: [],
      encryption: { enabled: true },
      replication: { factor: 3, strategy: 'full' },
      discovery: { mdns: true, bootstrap: false }
    });

    await manager1.leaveNetwork(networkId);

    const networks = manager1.getNetworks();
    expect(networks).toHaveLength(0);
  });

  it('should get network statistics', async () => {
    await manager1.initialize();
    const networkId = await manager1.createNetwork({
      networkId: 'test-network',
      name: 'Test Network',
      bootstrapPeers: [],
      encryption: { enabled: true },
      replication: { factor: 3, strategy: 'full' },
      discovery: { mdns: true, bootstrap: false }
    });

    const stats = manager1.getNetworkStats(networkId);

    expect(stats).not.toBeNull();
    expect(stats!.networkId).toBe(networkId);
    expect(stats!.connectedPeers).toBe(0);
    expect(stats!.operationsSent).toBe(0);
  });

  it('should register and handle message', async () => {
    await manager1.initialize();

    let receivedMessage = false;

    manager1.onMessage(MessageType.HEARTBEAT, async (msg) => {
      receivedMessage = true;
      expect(msg.type).toBe(MessageType.HEARTBEAT);
    });

    // Emit a message event directly for testing
    manager1.emit('message:received', {
      type: MessageType.HEARTBEAT,
      networkId: 'test',
      senderId: 'peer1',
      timestamp: Date.now(),
      payload: {}
    });

    // Wait for async handler
    await new Promise(resolve => setTimeout(resolve, 100));

    expect(receivedMessage).toBe(true);
  });

  it('should emit network created event', async () => {
    await manager1.initialize();

    let eventEmitted = false;

    manager1.once('network:created', (data) => {
      eventEmitted = true;
      expect(data.networkId).toBe('test-network');
    });

    await manager1.createNetwork({
      networkId: 'test-network',
      name: 'Test Network',
      bootstrapPeers: [],
      encryption: { enabled: true },
      replication: { factor: 3, strategy: 'full' },
      discovery: { mdns: true, bootstrap: false }
    });

    expect(eventEmitted).toBe(true);
  });

  it('should handle multiple networks', async () => {
    await manager1.initialize();

    await manager1.createNetwork({
      networkId: 'network1',
      name: 'Network 1',
      bootstrapPeers: [],
      encryption: { enabled: true },
      replication: { factor: 3, strategy: 'full' },
      discovery: { mdns: true, bootstrap: false }
    });

    await manager1.createNetwork({
      networkId: 'network2',
      name: 'Network 2',
      bootstrapPeers: [],
      encryption: { enabled: true },
      replication: { factor: 3, strategy: 'full' },
      discovery: { mdns: true, bootstrap: false }
    });

    const networks = manager1.getNetworks();
    expect(networks).toHaveLength(2);
    expect(networks.map(n => n.networkId)).toContain('network1');
    expect(networks.map(n => n.networkId)).toContain('network2');
  });
});
```

### Test Suite 4: Distributed Collection Integration Tests

**File:** `packages/core/src/distributed/tests/distributed-collection.test.ts`

```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { NetworkManager } from '../network-manager';
import { DistributedCollection } from '../distributed-collection';

describe('DistributedCollection', () => {
  let networkManager: NetworkManager;
  let collection: DistributedCollection;
  let networkId: string;

  beforeEach(async () => {
    networkManager = new NetworkManager();
    await networkManager.initialize();

    networkId = await networkManager.createNetwork({
      networkId: 'test-network',
      name: 'Test Network',
      bootstrapPeers: [],
      encryption: { enabled: true },
      replication: { factor: 3, strategy: 'full' },
      discovery: { mdns: true, bootstrap: false }
    });

    collection = new DistributedCollection('users', networkManager);
  });

  afterEach(async () => {
    await networkManager.shutdown();
  });

  it('should create a distributed collection', () => {
    expect(collection.name).toBe('users');
  });

  it('should attach collection to network', async () => {
    await collection.attachToNetwork(networkId);

    const syncState = collection.getSyncState();
    expect(syncState).not.toBeNull();
    expect(syncState!.networkId).toBe(networkId);
  });

  it('should detach collection from network', async () => {
    await collection.attachToNetwork(networkId);
    await collection.detachFromNetwork();

    const syncState = collection.getSyncState();
    expect(syncState).toBeNull();
  });

  it('should insert document and track in operation log', async () => {
    await collection.attachToNetwork(networkId);

    const doc = await collection.insert({ name: 'Alice', age: 30 });

    expect(doc.id).toBeTruthy();
    expect(doc.name).toBe('Alice');
    expect(doc.age).toBe(30);
  });

  it('should update document with distributed tracking', async () => {
    await collection.attachToNetwork(networkId);

    const doc = await collection.insert({ name: 'Bob', age: 25 });
    const updated = await collection.update({ id: doc.id }, { $set: { age: 26 } });

    expect(updated).toBe(1);

    const found = await collection.findOne({ id: doc.id });
    expect(found!.age).toBe(26);
  });

  it('should delete document with distributed tracking', async () => {
    await collection.attachToNetwork(networkId);

    const doc = await collection.insert({ name: 'Charlie', age: 35 });
    const deleted = await collection.delete({ id: doc.id });

    expect(deleted).toBe(1);

    const found = await collection.findOne({ id: doc.id });
    expect(found).toBeNull();
  });

  it('should maintain local functionality without network', async () => {
    const doc = await collection.insert({ name: 'Dave', age: 40 });
    expect(doc.id).toBeTruthy();

    const found = await collection.findOne({ id: doc.id });
    expect(found).not.toBeNull();
    expect(found!.name).toBe('Dave');
  });

  it('should throw error when attaching to non-existent network', async () => {
    await expect(
      collection.attachToNetwork('non-existent-network')
    ).rejects.toThrow();
  });

  it('should throw error when already attached', async () => {
    await collection.attachToNetwork(networkId);

    await expect(
      collection.attachToNetwork(networkId)
    ).rejects.toThrow();
  });

  it('should handle batch operations', async () => {
    await collection.attachToNetwork(networkId);

    const docs = await collection.insertBatch([
      { name: 'User1', age: 20 },
      { name: 'User2', age: 30 },
      { name: 'User3', age: 40 }
    ]);

    expect(docs).toHaveLength(3);

    const all = await collection.find({});
    expect(all).toHaveLength(3);
  });

  it('should force synchronization', async () => {
    await collection.attachToNetwork(networkId);

    const syncStateBefore = collection.getSyncState();
    const lastSyncBefore = syncStateBefore!.lastSync;

    // Wait a bit
    await new Promise(resolve => setTimeout(resolve, 100));

    await collection.forceSync();

    // Note: In a real scenario with multiple peers, this would sync with them
    // For this test, we just verify the method executes without error
  });
});
```

### Test Suite 5: Distributed Database Integration Tests

**File:** `packages/core/src/distributed/tests/distributed-database.test.ts`

```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { DistributedDatabase, createDistributedDb } from '../distributed-database';
import { InMemoryAdapter } from '../../in-memory-adapter';

describe('DistributedDatabase', () => {
  let db: DistributedDatabase;

  beforeEach(() => {
    db = createDistributedDb({
      adapter: new InMemoryAdapter(),
      plugins: [],
      distributed: {
        enabled: true,
        networkId: 'test-db-network'
      }
    });
  });

  afterEach(async () => {
    await db.shutdown();
  });

  it('should create a distributed database', () => {
    expect(db).toBeTruthy();
    expect(db.getNetworkManager()).toBeTruthy();
  });

  it('should create a network', async () => {
    const networkId = await db.createNetwork({
      networkId: 'new-network',
      name: 'New Network',
      bootstrapPeers: []
    });

    expect(networkId).toBe('new-network');

    const networks = db.getNetworkManager().getNetworks();
    expect(networks.some(n => n.networkId === 'new-network')).toBe(true);
  });

  it('should create distributed collections', async () => {
    const collection = db.collection('users');
    expect(collection).toBeTruthy();
    expect(collection.name).toBe('users');
  });

  it('should add collection to network', async () => {
    const networkId = await db.createNetwork({
      networkId: 'test-network',
      name: 'Test Network',
      bootstrapPeers: []
    });

    const collection = db.collection('users');
    await db.addCollectionToNetwork(networkId, 'users');

    const collections = db.getNetworkManager().getNetworkCollections(networkId);
    expect(collections).toContain('users');
  });

  it('should remove collection from network', async () => {
    const networkId = await db.createNetwork({
      networkId: 'test-network',
      name: 'Test Network',
      bootstrapPeers: []
    });

    const collection = db.collection('users');
    await db.addCollectionToNetwork(networkId, 'users');
    await db.removeCollectionFromNetwork('users');

    const collections = db.getNetworkManager().getNetworkCollections(networkId);
    expect(collections).not.toContain('users');
  });

  it('should handle multiple collections in different networks', async () => {
    const network1 = await db.createNetwork({
      networkId: 'network1',
      name: 'Network 1',
      bootstrapPeers: []
    });

    const network2 = await db.createNetwork({
      networkId: 'network2',
      name: 'Network 2',
      bootstrapPeers: []
    });

    const users = db.collection('users');
    const posts = db.collection('posts');

    await db.addCollectionToNetwork(network1, 'users');
    await db.addCollectionToNetwork(network2, 'posts');

    const net1Collections = db.getNetworkManager().getNetworkCollections(network1);
    const net2Collections = db.getNetworkManager().getNetworkCollections(network2);

    expect(net1Collections).toContain('users');
    expect(net1Collections).not.toContain('posts');
    expect(net2Collections).toContain('posts');
    expect(net2Collections).not.toContain('users');
  });

  it('should work without distributed mode', () => {
    const localDb = createDistributedDb({
      adapter: new InMemoryAdapter(),
      plugins: [],
      distributed: {
        enabled: false
      }
    });

    const collection = localDb.collection('users');
    expect(collection).toBeTruthy();
  });

  it('should join existing network', async () => {
    const bootstrapPeers = ['/ip4/127.0.0.1/tcp/4001/p2p/QmExamplePeer'];

    // This will attempt to join but won't connect in test environment
    // We just verify the method executes without error
    await db.joinNetwork('external-network', bootstrapPeers);

    const networks = db.getNetworkManager().getNetworks();
    expect(networks.some(n => n.networkId === 'external-network')).toBe(true);
  });

  it('should leave a network', async () => {
    const networkId = await db.createNetwork({
      networkId: 'temp-network',
      name: 'Temp Network',
      bootstrapPeers: []
    });

    await db.leaveNetwork(networkId);

    const networks = db.getNetworkManager().getNetworks();
    expect(networks.some(n => n.networkId === networkId)).toBe(false);
  });

  it('should throw error when adding non-existent collection to network', async () => {
    const networkId = await db.createNetwork({
      networkId: 'test-network',
      name: 'Test Network',
      bootstrapPeers: []
    });

    await expect(
      db.addCollectionToNetwork(networkId, 'non-existent')
    ).rejects.toThrow();
  });

  it('should maintain data integrity across operations', async () => {
    const collection = db.collection('users');

    await collection.insert({ name: 'Alice', age: 30 });
    await collection.insert({ name: 'Bob', age: 25 });

    const all = await collection.find({});
    expect(all).toHaveLength(2);

    await collection.update({ name: 'Alice' }, { $set: { age: 31 } });

    const alice = await collection.findOne({ name: 'Alice' });
    expect(alice!.age).toBe(31);

    await collection.delete({ name: 'Bob' });

    const remaining = await collection.find({});
    expect(remaining).toHaveLength(1);
  });
});
```

## Dependencies

Add to `packages/core/package.json`:

```json
{
  "dependencies": {
    "libp2p": "^1.0.0",
    "@libp2p/tcp": "^9.0.0",
    "@chainsafe/libp2p-noise": "^15.0.0",
    "@libp2p/mplex": "^10.0.0",
    "@libp2p/bootstrap": "^10.0.0",
    "@libp2p/mdns": "^10.0.0",
    "@libp2p/kad-dht": "^12.0.0",
    "@libp2p/interface-peer-id": "^4.0.0",
    "@multiformats/multiaddr": "^12.0.0",
    "@libp2p/peer-id": "^4.0.0",
    "it-pipe": "^3.0.0",
    "it-pushable": "^3.0.0"
  }
}
```

## Implementation Checklist

- [ ] Implement vector clock manager
- [ ] Implement CRDT resolver with conflict resolution
- [ ] Implement network manager with libp2p
- [ ] Implement distributed collection
- [ ] Implement distributed database
- [ ] Write comprehensive unit tests for all components
- [ ] Write integration tests for multi-peer scenarios
- [ ] Update documentation with distributed usage examples
- [ ] Add network monitoring and debugging tools
- [ ] Implement encryption for sensitive data
- [ ] Add performance benchmarks for distributed operations

## Usage Example

```typescript
import { createDistributedDb } from '@nebulus-db/core/distributed';
import { InMemoryAdapter } from '@nebulus-db/core';

// Create a distributed database
const db = createDistributedDb({
  adapter: new InMemoryAdapter(),
  distributed: {
    enabled: true,
    networkId: 'my-app-network'
  }
});

// Create a network
const networkId = await db.createNetwork({
  networkId: 'consortium-1',
  name: 'Company Consortium',
  bootstrapPeers: []
});

// Create collections
const users = db.collection('users');
const orders = db.collection('orders');

// Attach collections to network
await db.addCollectionToNetwork(networkId, 'users');
await db.addCollectionToNetwork(networkId, 'orders');

// Normal operations work as before
await users.insert({ name: 'Alice', email: 'alice@example.com' });
await orders.insert({ userId: 'alice-id', total: 100 });

// Data syncs automatically across all peers in the network

// Get network statistics
const stats = db.getNetworkManager().getNetworkStats(networkId);
console.log(`Connected peers: ${stats.connectedPeers}`);

// Remove collection from network
await db.removeCollectionFromNetwork('orders');

// Shutdown
await db.shutdown();
```

## Security Considerations

1. **Network Encryption**: All peer-to-peer communication uses TLS 1.3 via libp2p's noise protocol
2. **Shared Secrets**: Networks can optionally require shared secrets for joining
3. **Collection Access Control**: Only peers with access to a network can sync its collections
4. **Data Validation**: All incoming operations are validated before application
5. **DoS Protection**: Rate limiting on message processing to prevent flooding

## Performance Optimization

1. **Operation Log Pruning**: Keep only recent operations (configurable max size)
2. **Partial Replication**: Support for partial replication strategies
3. **Lazy Synchronization**: Sync only when needed, not on every operation
4. **Compression**: Compress large documents before transmission
5. **Batch Operations**: Batch multiple operations into single messages

## Future Enhancements

1. **Leader Election**: Implement Raft consensus for leader-based replication
2. **Sharding**: Distribute collections across multiple nodes for scalability
3. **WebRTC Support**: Add WebRTC transport for browser-to-browser connections
4. **Merkle Trees**: Use Merkle trees for efficient synchronization checks
5. **Garbage Collection**: Implement tombstone garbage collection for deleted documents
