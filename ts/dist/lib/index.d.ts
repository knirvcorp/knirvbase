export * from '../components/types/types';
export * from '../components/clock/vector_clock';
export * from '../components/collection/distributed_collection';
export * from '../components/network/network_manager';
export * from '../components/storage/storage';
export * from '../components/storage/index';
export * from '../components/resolver/crdt_resolver';
export * from '../components/crypto/pqc/keys';
export * from '../components/crypto/pqc/encryption';
export * from '../components/query/index';
import { NetworkConfig } from '../components/types/types';
export interface Options {
    dataDir: string;
    distributedEnabled: boolean;
    distributedNetworkID?: string;
    distributedBootstrapPeers?: string[];
}
export declare class DB {
    private db;
    private store;
    private network;
    private collections;
    constructor(options: Options);
    initialize(): Promise<void>;
    createNetwork(cfg: NetworkConfig): Promise<string>;
    joinNetwork(networkID: string, bootstrapPeers: string[]): Promise<void>;
    leaveNetwork(networkID: string): Promise<void>;
    collection(name: string): Collection;
    shutdown(): Promise<void>;
}
export interface Collection {
    insert(doc: Record<string, any>): Promise<Record<string, any>>;
    update(id: string, update: Record<string, any>): Promise<number>;
    delete(id: string): Promise<number>;
    find(id: string): Promise<Record<string, any> | null>;
    findAll(): Promise<Record<string, any>[]>;
    attachToNetwork(networkID: string): Promise<void>;
    detachFromNetwork(): Promise<void>;
    forceSync(): Promise<void>;
}
export declare function New(ctx: any, opts: Options): Promise<DB>;
//# sourceMappingURL=index.d.ts.map