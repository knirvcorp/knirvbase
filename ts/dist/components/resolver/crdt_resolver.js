"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.ResolveConflict = ResolveConflict;
exports.ApplyOperation = ApplyOperation;
exports.ToDistributed = ToDistributed;
exports.ToRegular = ToRegular;
const vector_clock_1 = require("../clock/vector_clock");
const types_1 = require("../types/types");
// ResolveConflict applies LWW + vector-clock tie-breaking
function ResolveConflict(local, remote) {
    if (!remote)
        return local;
    if (!local)
        return remote;
    // Deletion handling
    if (remote._deleted && !local._deleted) {
        const comp = (0, vector_clock_1.compare)(local._vector, remote._vector);
        if (comp === vector_clock_1.ComparisonResult.Before || comp === vector_clock_1.ComparisonResult.Concurrent) {
            return remote;
        }
        return local;
    }
    if (local._deleted && !remote._deleted) {
        const comp = (0, vector_clock_1.compare)(remote._vector, local._vector);
        if (comp === vector_clock_1.ComparisonResult.Before || comp === vector_clock_1.ComparisonResult.Concurrent) {
            return local;
        }
        return remote;
    }
    switch ((0, vector_clock_1.compare)(local._vector, remote._vector)) {
        case vector_clock_1.ComparisonResult.After:
            return local;
        case vector_clock_1.ComparisonResult.Before:
            return remote;
        case vector_clock_1.ComparisonResult.Equal:
            return local;
        case vector_clock_1.ComparisonResult.Concurrent:
            // Use timestamp, then peer id for deterministic ordering
            if (local._timestamp > remote._timestamp) {
                return mergeDocuments(local, remote);
            }
            else if (local._timestamp < remote._timestamp) {
                return mergeDocuments(remote, local);
            }
            if (local._peerId >= remote._peerId) {
                return mergeDocuments(local, remote);
            }
            return mergeDocuments(remote, local);
    }
    return local;
}
function mergeDocuments(winner, loser) {
    const merged = { ...winner };
    merged._vector = (0, vector_clock_1.merge)(winner._vector, loser._vector);
    // Merge non-conflicting fields from loser payload
    if (!merged.payload) {
        merged.payload = {};
    }
    for (const [k, v] of Object.entries(loser.payload || {})) {
        if (!(k in merged.payload)) {
            merged.payload[k] = v;
        }
    }
    return merged;
}
// ApplyOperation applies a CRDT operation to a document (insert/update/delete)
function ApplyOperation(doc, op) {
    switch (op.type) {
        case types_1.OperationType.Insert:
        case types_1.OperationType.Update:
            if (!doc) {
                if (!op.data)
                    return null;
                const copy = { ...op.data };
                copy._vector = (0, vector_clock_1.clone)(op.vector);
                copy._timestamp = op.timestamp;
                copy._peerId = op.peerId;
                return copy;
            }
            const comp = (0, vector_clock_1.compare)(doc._vector, op.vector);
            if (comp === vector_clock_1.ComparisonResult.Before || comp === vector_clock_1.ComparisonResult.Concurrent) {
                // Merge fields
                if (!doc.payload)
                    doc.payload = {};
                for (const [k, v] of Object.entries(op.data?.payload || {})) {
                    doc.payload[k] = v;
                }
                doc._vector = (0, vector_clock_1.merge)(doc._vector, op.vector);
                if (op.timestamp > doc._timestamp) {
                    doc._timestamp = op.timestamp;
                }
            }
            return doc;
        case types_1.OperationType.Delete:
            if (!doc)
                return null;
            const compDel = (0, vector_clock_1.compare)(doc._vector, op.vector);
            if (compDel === vector_clock_1.ComparisonResult.Before || compDel === vector_clock_1.ComparisonResult.Concurrent) {
                doc._deleted = true;
                doc._vector = (0, vector_clock_1.merge)(doc._vector, op.vector);
                if (op.timestamp > doc._timestamp) {
                    doc._timestamp = op.timestamp;
                }
            }
            return doc;
        default:
            return doc;
    }
}
// ToDistributed converts a simple map payload to DistributedDocument
function ToDistributed(payload, peerID) {
    const v = {};
    v[peerID] = 1;
    return {
        id: payload.id,
        entryType: payload.entryType,
        payload: cloneMap(payload),
        _vector: v,
        _timestamp: Date.now(),
        _peerId: peerID,
    };
}
// ToRegular converts a DistributedDocument to a regular map representation
function ToRegular(doc) {
    if (!doc)
        return null;
    return cloneMap(doc.payload) || null;
}
// cloneMap creates a shallow copy of a map
function cloneMap(src) {
    if (!src)
        return undefined;
    return { ...src };
}
//# sourceMappingURL=crdt_resolver.js.map