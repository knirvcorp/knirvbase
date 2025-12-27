import { VectorClock, compare, merge, clone, ComparisonResult } from '../clock/vector_clock';
import { DistributedDocument, CRDTOperation, OperationType } from '../types/types';

// ResolveConflict applies LWW + vector-clock tie-breaking
export function ResolveConflict(local: DistributedDocument | null, remote: DistributedDocument | null): DistributedDocument | null {
  if (!remote) return local;
  if (!local) return remote;

  // Deletion handling
  if (remote._deleted && !local._deleted) {
    const comp = compare(local._vector, remote._vector);
    if (comp === ComparisonResult.Before || comp === ComparisonResult.Concurrent) {
      return remote;
    }
    return local;
  }
  if (local._deleted && !remote._deleted) {
    const comp = compare(remote._vector, local._vector);
    if (comp === ComparisonResult.Before || comp === ComparisonResult.Concurrent) {
      return local;
    }
    return remote;
  }

  switch (compare(local._vector, remote._vector)) {
    case ComparisonResult.After:
      return local;
    case ComparisonResult.Before:
      return remote;
    case ComparisonResult.Equal:
      return local;
    case ComparisonResult.Concurrent:
      // Use timestamp, then peer id for deterministic ordering
      if (local._timestamp > remote._timestamp) {
        return mergeDocuments(local, remote);
      } else if (local._timestamp < remote._timestamp) {
        return mergeDocuments(remote, local);
      }
      if (local._peerId >= remote._peerId) {
        return mergeDocuments(local, remote);
      }
      return mergeDocuments(remote, local);
  }
  return local;
}

function mergeDocuments(winner: DistributedDocument, loser: DistributedDocument): DistributedDocument {
  const merged: DistributedDocument = { ...winner };
  merged._vector = merge(winner._vector, loser._vector);

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
export function ApplyOperation(doc: DistributedDocument | null, op: CRDTOperation): DistributedDocument | null {
  switch (op.type) {
    case OperationType.Insert:
    case OperationType.Update:
      if (!doc) {
        if (!op.data) return null;
        const copy: DistributedDocument = { ...op.data };
        copy._vector = clone(op.vector);
        copy._timestamp = op.timestamp;
        copy._peerId = op.peerId;
        return copy;
      }

      const comp = compare(doc._vector, op.vector);
      if (comp === ComparisonResult.Before || comp === ComparisonResult.Concurrent) {
        // Merge fields
        if (!doc.payload) doc.payload = {};
        for (const [k, v] of Object.entries(op.data?.payload || {})) {
          doc.payload[k] = v;
        }
        doc._vector = merge(doc._vector, op.vector);
        if (op.timestamp > doc._timestamp) {
          doc._timestamp = op.timestamp;
        }
      }
      return doc;

    case OperationType.Delete:
      if (!doc) return null;
      const compDel = compare(doc._vector, op.vector);
      if (compDel === ComparisonResult.Before || compDel === ComparisonResult.Concurrent) {
        doc._deleted = true;
        doc._vector = merge(doc._vector, op.vector);
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
export function ToDistributed(payload: Record<string, any>, peerID: string): DistributedDocument {
  const v: VectorClock = {};
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
export function ToRegular(doc: DistributedDocument | null): Record<string, any> | null {
  if (!doc) return null;
  return cloneMap(doc.payload) || null;
}

// cloneMap creates a shallow copy of a map
function cloneMap(src: Record<string, any> | undefined): Record<string, any> | undefined {
  if (!src) return undefined;
  return { ...src };
}