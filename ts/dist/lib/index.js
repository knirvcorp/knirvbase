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
export class DB {
    constructor(options) {
        this.collections = new Map();
        this.store = new FileStorage(options.dataDir);
        this.network = new NetworkManager();
        // Initialize network if distributed
        if (options.distributedEnabled) {
            // TODO: Initialize distributed database
        }
    }
    async initialize() {
        await this.network.initialize();
    }
    async createNetwork(cfg) {
        return this.network.createNetwork(cfg);
    }
    async joinNetwork(networkID, bootstrapPeers) {
        await this.network.joinNetwork(networkID, bootstrapPeers);
    }
    async leaveNetwork(networkID) {
        await this.network.leaveNetwork(networkID);
    }
    collection(name) {
        if (this.collections.has(name)) {
            return new CollectionAdapter(this.collections.get(name));
        }
        const coll = new DistributedCollection(name, this.network, this.store);
        this.collections.set(name, coll);
        return new CollectionAdapter(coll);
    }
    async shutdown() {
        await this.network.shutdown();
    }
}
class CollectionAdapter {
    constructor(coll) {
        this.coll = coll;
    }
    async insert(doc) {
        return this.coll.insert(doc);
    }
    async update(id, update) {
        return this.coll.update(id, update);
    }
    async delete(id) {
        return this.coll.delete(id);
    }
    async find(id) {
        return this.coll.find(id);
    }
    async findAll() {
        return this.coll.findAll();
    }
    async attachToNetwork(networkID) {
        await this.coll.attachToNetwork(networkID);
    }
    async detachFromNetwork() {
        await this.coll.detachFromNetwork();
    }
    async forceSync() {
        await this.coll.forceSync();
    }
}
// Factory function
export async function New(ctx, opts) {
    const db = new DB(opts);
    await db.initialize();
    return db;
}
//# sourceMappingURL=index.js.map