package resolver

import (
	"time"

	"github.com/knirv/knirvbase/internal/clock"
	"github.com/knirv/knirvbase/internal/types"
)

// ResolveConflict applies LWW + vector-clock tie-breaking
func ResolveConflict(local, remote *types.DistributedDocument) *types.DistributedDocument {
	if remote == nil {
		return local
	}
	if local == nil {
		return remote
	}

	// Deletion handling
	if remote.Deleted && !local.Deleted {
		comp := clock.Compare(local.Vector, remote.Vector)
		if comp == clock.Before || comp == clock.Concurrent {
			return remote
		}
		return local
	}
	if local.Deleted && !remote.Deleted {
		comp := clock.Compare(remote.Vector, local.Vector)
		if comp == clock.Before || comp == clock.Concurrent {
			return local
		}
		return remote
	}

	switch clock.Compare(local.Vector, remote.Vector) {
	case clock.After:
		return local
	case clock.Before:
		return remote
	case clock.Equal:
		return local
	case clock.Concurrent:
		// Use timestamp, then peer id for deterministic ordering
		if local.Timestamp > remote.Timestamp {
			return mergeDocuments(local, remote)
		} else if local.Timestamp < remote.Timestamp {
			return mergeDocuments(remote, local)
		}
		if local.PeerID >= remote.PeerID {
			return mergeDocuments(local, remote)
		}
		return mergeDocuments(remote, local)
	}
	return local
}

func mergeDocuments(winner, loser *types.DistributedDocument) *types.DistributedDocument {
	merged := *winner // shallow copy
	merged.Vector = clock.Merge(winner.Vector, loser.Vector)

	// Merge non-conflicting fields from loser payload
	if merged.Payload == nil {
		merged.Payload = make(map[string]interface{})
	}
	for k, v := range loser.Payload {
		if _, ok := merged.Payload[k]; !ok {
			merged.Payload[k] = v
		}
	}
	return &merged
}

// ApplyOperation applies a CRDT operation to a document (insert/update/delete)
// Note: Documents marked with Stage == "post-pending" are treated as a local intent
// and are not emitted as regular CRDT operations until they are explicitly posted as
// KNIRVGRAPH transactions during the sync flow.
func ApplyOperation(doc *types.DistributedDocument, op types.CRDTOperation) *types.DistributedDocument {
	switch op.Type {
	case types.OpInsert, types.OpUpdate:
		if doc == nil {
			if op.Data == nil {
				return nil
			}
			copy := *op.Data
			copy.Vector = clock.Clone(op.Vector)
			copy.Timestamp = op.Timestamp
			copy.PeerID = op.PeerID
			return &copy
		}

		comp := clock.Compare(doc.Vector, op.Vector)
		if comp == clock.Before || comp == clock.Concurrent {
			// Merge fields
			if doc.Payload == nil {
				doc.Payload = make(map[string]interface{})
			}
			for k, v := range op.Data.Payload {
				doc.Payload[k] = v
			}
			doc.Vector = clock.Merge(doc.Vector, op.Vector)
			if op.Timestamp > doc.Timestamp {
				doc.Timestamp = op.Timestamp
			}
		}
		return doc

	case types.OpDelete:
		if doc == nil {
			return nil
		}
		comp := clock.Compare(doc.Vector, op.Vector)
		if comp == clock.Before || comp == clock.Concurrent {
			doc.Deleted = true
			doc.Vector = clock.Merge(doc.Vector, op.Vector)
			if op.Timestamp > doc.Timestamp {
				doc.Timestamp = op.Timestamp
			}
		}
		return doc
	default:
		return doc
	}
}

// ToDistributed converts a simple map payload to DistributedDocument
func ToDistributed(payload map[string]interface{}, peerID string) *types.DistributedDocument {
	v := clock.NewVectorClock()
	v[peerID] = 1
	return &types.DistributedDocument{
		ID:        payload["id"].(string),
		Payload:   cloneMap(payload),
		Vector:    v,
		Timestamp: time.Now().UnixMilli(),
		PeerID:    peerID,
	}
}

// ToRegular converts a DistributedDocument to a regular map representation
func ToRegular(doc *types.DistributedDocument) map[string]interface{} {
	if doc == nil {
		return nil
	}
	return cloneMap(doc.Payload)
}

// cloneMap creates a shallow copy of a map
func cloneMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
