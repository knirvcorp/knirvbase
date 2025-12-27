import { VectorClock } from '../clock/vector_clock';
export declare enum EntryType {
    Memory = "MEMORY",
    Auth = "AUTH"
}
export interface DistributedDocument {
    id: string;
    entryType: EntryType;
    payload?: Record<string, any>;
    _vector: VectorClock;
    _timestamp: number;
    _peerId: string;
    _stage?: string;
    _deleted?: boolean;
}
export declare enum OperationType {
    Insert = 0,
    Update = 1,
    Delete = 2
}
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
export interface NetworkConfig {
    networkId: string;
    name: string;
    collections: Record<string, boolean>;
    bootstrapPeers: string[];
    defaultPostingNetwork: string;
    autoPostClassifications: EntryType[];
    privateByDefault: boolean;
    encryption: {
        enabled: boolean;
        sharedSecret: string;
    };
    replication: {
        factor: number;
        strategy: string;
    };
    discovery: {
        mdns: boolean;
        bootstrap: boolean;
    };
}
export interface PeerInfo {
    peerId: string;
    addrs: string[];
    protocols: string[];
    latency: number;
    lastSeen: Date;
    collections: string[];
}
export interface SyncState {
    collection: string;
    networkId: string;
    localVector: VectorClock;
    lastSync: Date;
    pendingOperations: CRDTOperation[];
    stagedEntries: string[];
    syncInProgress: boolean;
}
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
export declare enum MessageType {
    SyncRequest = "sync_request",
    SyncResponse = "sync_response",
    Operation = "operation",
    Heartbeat = "heartbeat",
    CollectionAnnounce = "collection_announce",
    CollectionRequest = "collection_request"
}
export interface ProtocolMessage {
    type: MessageType;
    networkId: string;
    senderId: string;
    timestamp: number;
    payload: any;
}
//# sourceMappingURL=types.d.ts.map