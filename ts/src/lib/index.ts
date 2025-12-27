// Main library exports for KNIRVBASE

// Core types
export * from '../components/types/types';

// Clock
export * from '../components/clock/vector_clock';

// Collection
export * from '../components/collection/distributed_collection';

// Network
export * from '../components/network/network_manager';

// Storage
export * from '../components/storage/storage';
export * from '../components/storage/index';

// Resolver
export * from '../components/resolver/crdt_resolver';

// Crypto
export * from '../components/crypto/pqc/keys';
export * from '../components/crypto/pqc/encryption';

// Query
export * from '../components/query/index';

// Main Database class
import { NetworkManager } from '../components/network/network_manager';
import { FileStorage } from '../components/storage/storage';
import { DistributedCollection } from '../components/collection/distributed_collection';
import { NetworkConfig } from '../components/types/types';

export interface Options {
  dataDir: string;
  distributedEnabled: boolean;
  distributedNetworkID?: string;
  distributedBootstrapPeers?: string[];
}

export class DB {
  private db: any; // Placeholder for internal database if needed
  private store: FileStorage;
  private network: NetworkManager;
  private collections: Map<string, DistributedCollection> = new Map();

  constructor(options: Options) {
    this.store = new FileStorage(options.dataDir);
    this.network = new NetworkManager();
    // Initialize network if distributed
    if (options.distributedEnabled) {
      // TODO: Initialize distributed database
    }
  }

  async initialize(): Promise<void> {
    await this.network.initialize();
  }

  async createNetwork(cfg: NetworkConfig): Promise<string> {
    return this.network.createNetwork(cfg);
  }

  async joinNetwork(networkID: string, bootstrapPeers: string[]): Promise<void> {
    await this.network.joinNetwork(networkID, bootstrapPeers);
  }

  async leaveNetwork(networkID: string): Promise<void> {
    await this.network.leaveNetwork(networkID);
  }

  collection(name: string): Collection {
    if (this.collections.has(name)) {
      return new CollectionAdapter(this.collections.get(name)!);
    }
    const coll = new DistributedCollection(name, this.network, this.store);
    this.collections.set(name, coll);
    return new CollectionAdapter(coll);
  }

  async shutdown(): Promise<void> {
    await this.network.shutdown();
  }
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

class CollectionAdapter implements Collection {
  constructor(private coll: DistributedCollection) {}

  async insert(doc: Record<string, any>): Promise<Record<string, any>> {
    return this.coll.insert(doc);
  }

  async update(id: string, update: Record<string, any>): Promise<number> {
    return this.coll.update(id, update);
  }

  async delete(id: string): Promise<number> {
    return this.coll.delete(id);
  }

  async find(id: string): Promise<Record<string, any> | null> {
    return this.coll.find(id);
  }

  async findAll(): Promise<Record<string, any>[]> {
    return this.coll.findAll();
  }

  async attachToNetwork(networkID: string): Promise<void> {
    await this.coll.attachToNetwork(networkID);
  }

  async detachFromNetwork(): Promise<void> {
    await this.coll.detachFromNetwork();
  }

  async forceSync(): Promise<void> {
    await this.coll.forceSync();
  }
}

// Factory function
export async function New(ctx: any, opts: Options): Promise<DB> {
  const db = new DB(opts);
  await db.initialize();
  return db;
}