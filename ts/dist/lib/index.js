"use strict";
// Main library exports for KNIRVBASE
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __exportStar = (this && this.__exportStar) || function(m, exports) {
    for (var p in m) if (p !== "default" && !Object.prototype.hasOwnProperty.call(exports, p)) __createBinding(exports, m, p);
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.DB = void 0;
exports.New = New;
// Core types
__exportStar(require("../components/types/types"), exports);
// Clock
__exportStar(require("../components/clock/vector_clock"), exports);
// Collection
__exportStar(require("../components/collection/distributed_collection"), exports);
// Network
__exportStar(require("../components/network/network_manager"), exports);
// Storage
__exportStar(require("../components/storage/storage"), exports);
__exportStar(require("../components/storage/index"), exports);
// Resolver
__exportStar(require("../components/resolver/crdt_resolver"), exports);
// Crypto
__exportStar(require("../components/crypto/pqc/keys"), exports);
__exportStar(require("../components/crypto/pqc/encryption"), exports);
// Query
__exportStar(require("../components/query/index"), exports);
// Main Database class
const network_manager_1 = require("../components/network/network_manager");
const storage_1 = require("../components/storage/storage");
const distributed_collection_1 = require("../components/collection/distributed_collection");
class DB {
    constructor(options) {
        this.collections = new Map();
        this.store = new storage_1.FileStorage(options.dataDir);
        this.network = new network_manager_1.NetworkManager();
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
        const coll = new distributed_collection_1.DistributedCollection(name, this.network, this.store);
        this.collections.set(name, coll);
        return new CollectionAdapter(coll);
    }
    async shutdown() {
        await this.network.shutdown();
    }
}
exports.DB = DB;
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
async function New(ctx, opts) {
    const db = new DB(opts);
    await db.initialize();
    return db;
}
//# sourceMappingURL=index.js.map