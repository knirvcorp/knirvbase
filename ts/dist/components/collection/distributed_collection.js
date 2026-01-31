import { increment, merge } from '../clock/vector_clock';
import { OperationType, MessageType, EntryType } from '../types/types';
import { ToDistributed, ToRegular, ApplyOperation } from '../resolver/crdt_resolver';
// A minimal in-memory local collection implementation to keep the example self-contained
export class LocalCollection {
    constructor(name, store) {
        this.name = name;
        this.store = store;
    }
    async insert(doc) {
        // Store a deep-cloned copy to avoid accidental shared references between
        // callers and the underlying storage implementation.
        const cloned = this.cloneMap(doc);
        await this.store.insert(this.name, cloned);
        // Return another clone so the caller cannot mutate the stored value
        return this.cloneMap(cloned);
    }
    async update(id, update) {
        return await this.store.update(this.name, id, update);
    }
    async delete(id) {
        return await this.store.delete(this.name, id);
    }
    async find(id) {
        return await this.store.find(this.name, id);
    }
    async findAll() {
        return await this.store.findAll(this.name);
    }
    cloneMap(m) {
        const out = {};
        for (const k in m) {
            const v = m[k];
            if (typeof v === 'object' && v !== null && !Array.isArray(v)) {
                out[k] = this.cloneMap(v);
            }
            else if (Array.isArray(v)) {
                out[k] = this.cloneSlice(v);
            }
            else {
                // primitives and unknown types are copied by value/reference as-is
                out[k] = v;
            }
        }
        return out;
    }
    cloneSlice(s) {
        const out = [];
        for (const e of s) {
            if (typeof e === 'object' && e !== null && !Array.isArray(e)) {
                out.push(this.cloneMap(e));
            }
            else if (Array.isArray(e)) {
                out.push(this.cloneSlice(e));
            }
            else {
                out.push(e);
            }
        }
        return out;
    }
}
// DistributedCollection manages local storage plus network synchronization
export class DistributedCollection {
    constructor(name, network, store) {
        this.networkID = '';
        this.syncStates = new Map();
        this.operationLog = [];
        this.maxLogSize = 10000;
        this.name = name;
        this.network = network;
        this.local = new LocalCollection(name, store);
        this.setupMessageHandlers();
    }
    setupMessageHandlers() {
        this.network.onMessage(MessageType.Operation, (msg) => {
            // Basic payload validation
            const payload = msg.payload;
            if (!payload)
                return;
            const coll = payload.collection;
            if (coll !== this.name)
                return;
            const opMap = payload.operation;
            // We assume op was encoded in a friendly form; in a real implementation we'd use typed marshaling
            const op = opMap; // Simplified
            this.handleRemoteOperation(op);
        });
        this.network.onMessage(MessageType.SyncRequest, (msg) => {
            const payload = msg.payload;
            if (!payload)
                return;
            const coll = payload.collection;
            if (coll !== this.name)
                return;
            this.handleSyncRequest(msg);
        });
        this.network.onMessage(MessageType.SyncResponse, (msg) => {
            const payload = msg.payload;
            if (!payload)
                return;
            const coll = payload.collection;
            if (coll !== this.name)
                return;
            this.handleSyncResponse(msg);
        });
    }
    async attachToNetwork(networkID) {
        if (this.networkID !== '') {
            throw new Error(`collection ${this.name} already attached to ${this.networkID}`);
        }
        await this.network.addCollectionToNetwork(networkID, this.name);
        this.networkID = networkID;
        this.syncStates.set(networkID, {
            collection: this.name,
            networkId: networkID,
            localVector: {},
            lastSync: new Date(),
            pendingOperations: [],
            stagedEntries: [],
            syncInProgress: false
        });
        // Request initial sync
        await this.requestSync();
    }
    async detachFromNetwork() {
        if (this.networkID === '')
            return;
        await this.network.removeCollectionFromNetwork(this.networkID, this.name);
        this.syncStates.delete(this.networkID);
        this.networkID = '';
    }
    async insert(doc) {
        const id = doc.id;
        if (!id) {
            throw new Error("document must contain 'id'");
        }
        const entryType = doc.entryType; // Assuming validation already happened
        // For MEMORY entries, blob is handled by storage
        if (entryType === EntryType.Memory) {
            const payload = doc.payload;
            if (payload && 'blob' in payload) {
                // Blob will be saved by storage.insert
            }
        }
        // The local collection handles persistence. The underlying adapter is responsible
        // for the actual file I/O for blobs.
        const inserted = await this.local.insert(doc);
        if (this.networkID !== '') {
            const opPayload = ToDistributed(inserted, this.network.getPeerID());
            opPayload.entryType = entryType; // Ensure EntryType is set on the distributed doc
            const op = {
                id: `${this.network.getPeerID()}-${Date.now()}-${Math.random()}`,
                type: OperationType.Insert,
                collection: this.name,
                documentId: id,
                data: opPayload,
                vector: this.getCurrentVector(),
                timestamp: Date.now(),
                peerId: this.network.getPeerID(),
            };
            this.broadcastOperation(op);
        }
        return inserted;
    }
    async update(id, update) {
        const affected = await this.local.update(id, update);
        if (this.networkID !== '' && affected > 0) {
            const doc = await this.local.find(id);
            const op = {
                id: `${this.network.getPeerID()}-${Date.now()}`,
                type: OperationType.Update,
                collection: this.name,
                documentId: id,
                data: ToDistributed(doc, this.network.getPeerID()),
                vector: this.getCurrentVector(),
                timestamp: Date.now(),
                peerId: this.network.getPeerID()
            };
            this.broadcastOperation(op);
        }
        return affected;
    }
    async delete(id) {
        const affected = await this.local.delete(id);
        if (this.networkID !== '' && affected > 0) {
            const op = {
                id: `${this.network.getPeerID()}-${Date.now()}`,
                type: OperationType.Delete,
                collection: this.name,
                documentId: id,
                data: { id, _deleted: true },
                vector: this.getCurrentVector(),
                timestamp: Date.now(),
                peerId: this.network.getPeerID()
            };
            this.broadcastOperation(op);
        }
        return affected;
    }
    async find(id) {
        return await this.local.find(id);
    }
    async findAll() {
        return await this.local.findAll();
    }
    getSyncState() {
        if (this.networkID === '')
            return null;
        return this.syncStates.get(this.networkID) || null;
    }
    async forceSync() {
        await this.requestSync();
    }
    // Private helpers
    broadcastOperation(op) {
        if (this.networkID === '')
            return;
        this.operationLog.push(op);
        this.pruneOperationLog();
        // increment local vector
        const syncState = this.syncStates.get(this.networkID);
        syncState.localVector = increment(syncState.localVector, this.network.getPeerID());
        this.network.broadcastMessage(this.networkID, {
            type: MessageType.Operation,
            networkId: this.networkID,
            senderId: this.network.getPeerID(),
            timestamp: Date.now(),
            payload: { collection: this.name, operation: op }
        });
    }
    async handleRemoteOperation(op) {
        // Apply CRDT operation to local document
        const existing = await this.local.find(op.documentId);
        let existingDist = null;
        if (existing) {
            existingDist = ToDistributed(existing, op.peerId);
        }
        const result = ApplyOperation(existingDist, op);
        if (result === null) {
            // delete
            await this.local.delete(op.documentId);
        }
        else if (result._deleted) {
            await this.local.delete(op.documentId);
        }
        else {
            // upsert
            const regular = ToRegular(result);
            if (regular) {
                await this.local.insert(regular);
            }
        }
        // merge vector
        if (this.networkID !== '') {
            const syncState = this.syncStates.get(this.networkID);
            syncState.localVector = merge(syncState.localVector, op.vector);
        }
    }
    async requestSync() {
        if (this.networkID === '') {
            throw new Error('not attached to network');
        }
        const syncState = this.syncStates.get(this.networkID);
        if (syncState.syncInProgress)
            return;
        syncState.syncInProgress = true;
        await this.network.broadcastMessage(this.networkID, {
            type: MessageType.SyncRequest,
            networkId: this.networkID,
            senderId: this.network.getPeerID(),
            timestamp: Date.now(),
            payload: { collection: this.name, vector: syncState.localVector }
        });
        // Clear flag after timeout
        setTimeout(() => {
            syncState.syncInProgress = false;
        }, 10000);
    }
    handleSyncRequest(msg) {
        const payload = msg.payload;
        const remoteVector = payload.vector;
        // find missing ops
        const missing = [];
        for (const op of this.operationLog) {
            const remoteClock = remoteVector[op.peerId] || 0;
            const opClock = op.vector[op.peerId] || 0;
            if (opClock > remoteClock) {
                missing.push(op);
            }
        }
        this.network.sendToPeer(msg.senderId, this.networkID, {
            type: MessageType.SyncResponse,
            networkId: this.networkID,
            senderId: this.network.getPeerID(),
            timestamp: Date.now(),
            payload: { collection: this.name, operations: missing, vector: this.syncStates.get(this.networkID).localVector }
        });
    }
    handleSyncResponse(msg) {
        const payload = msg.payload;
        const opsIface = payload.operations;
        for (const oi of opsIface) {
            this.handleRemoteOperation(oi);
        }
        if (this.networkID !== '') {
            const syncState = this.syncStates.get(this.networkID);
            syncState.syncInProgress = false;
            syncState.lastSync = new Date();
        }
    }
    getCurrentVector() {
        if (this.networkID === '')
            return {};
        const s = this.syncStates.get(this.networkID);
        return s ? { ...s.localVector } : {};
    }
    pruneOperationLog() {
        if (this.operationLog.length > this.maxLogSize) {
            this.operationLog = this.operationLog.slice(this.operationLog.length - this.maxLogSize);
        }
    }
}
//# sourceMappingURL=distributed_collection.js.map