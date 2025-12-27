import { Network } from '../network/network_manager';
import { Storage } from '../storage/storage';
import { SyncState } from '../types/types';
export declare class LocalCollection {
    private name;
    private store;
    constructor(name: string, store: Storage);
    insert(doc: Record<string, any>): Promise<Record<string, any>>;
    update(id: string, update: Record<string, any>): Promise<number>;
    delete(id: string): Promise<number>;
    find(id: string): Promise<Record<string, any> | null>;
    findAll(): Promise<Record<string, any>[]>;
    private cloneMap;
    private cloneSlice;
}
export declare class DistributedCollection {
    name: string;
    private network;
    private networkID;
    private syncStates;
    private operationLog;
    private maxLogSize;
    private local;
    constructor(name: string, network: Network, store: Storage);
    private setupMessageHandlers;
    attachToNetwork(networkID: string): Promise<void>;
    detachFromNetwork(): Promise<void>;
    insert(doc: Record<string, any>): Promise<Record<string, any>>;
    update(id: string, update: Record<string, any>): Promise<number>;
    delete(id: string): Promise<number>;
    find(id: string): Promise<Record<string, any> | null>;
    findAll(): Promise<Record<string, any>[]>;
    getSyncState(): SyncState | null;
    forceSync(): Promise<void>;
    private broadcastOperation;
    private handleRemoteOperation;
    private requestSync;
    private handleSyncRequest;
    private handleSyncResponse;
    private getCurrentVector;
    private pruneOperationLog;
}
//# sourceMappingURL=distributed_collection.d.ts.map