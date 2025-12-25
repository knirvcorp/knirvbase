package distributed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/knirv/knirvbase/internal/clock"
	netpkg "github.com/knirv/knirvbase/internal/network"
	resolver "github.com/knirv/knirvbase/internal/resolver"
	stor "github.com/knirv/knirvbase/internal/storage"
	typ "github.com/knirv/knirvbase/internal/types"
)

// A minimal in-memory local collection implementation to keep the example self-contained
type LocalCollection struct {
	mu    sync.RWMutex
	name  string
	store stor.Storage
}

func NewLocalCollection(name string, store stor.Storage) *LocalCollection {
	return &LocalCollection{name: name, store: store}
}

func (c *LocalCollection) Insert(ctx context.Context, doc map[string]interface{}) (map[string]interface{}, error) {
	// Store a deep-cloned copy to avoid accidental shared references between
	// callers and the underlying storage implementation.
	cloned := cloneMap(doc)
	if err := c.store.Insert(c.name, cloned); err != nil {
		return nil, err
	}
	// Return another clone so the caller cannot mutate the stored value
	return cloneMap(cloned), nil
}

func (c *LocalCollection) Update(id string, update map[string]interface{}) (int, error) {
	return 1, c.store.Update(c.name, id, update)
}

func (c *LocalCollection) Delete(id string) (int, error) {
	return 1, c.store.Delete(c.name, id)
}

func (c *LocalCollection) Find(id string) (map[string]interface{}, error) {
	return c.store.Find(c.name, id)
}

func cloneMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case map[string]interface{}:
			out[k] = cloneMap(val)
		case []interface{}:
			out[k] = cloneSlice(val)
		default:
			// primitives and unknown types are copied by value/reference as-is
			out[k] = val
		}
	}
	return out
}

func cloneSlice(s []interface{}) []interface{} {
	if s == nil {
		return nil
	}
	out := make([]interface{}, len(s))
	for i, e := range s {
		switch v := e.(type) {
		case map[string]interface{}:
			out[i] = cloneMap(v)
		case []interface{}:
			out[i] = cloneSlice(v)
		default:
			out[i] = v
		}
	}
	return out
}

// DistributedCollection manages local storage plus network synchronization
type DistributedCollection struct {
	Name         string
	network      netpkg.Network
	networkID    string
	syncStates   map[string]*typ.SyncState
	operationLog []typ.CRDTOperation
	maxLogSize   int

	local *LocalCollection
	mu    sync.Mutex
}

func NewDistributedCollection(name string, net netpkg.Network, store stor.Storage) *DistributedCollection {
	dc := &DistributedCollection{
		Name:         name,
		network:      net,
		syncStates:   make(map[string]*typ.SyncState),
		operationLog: make([]typ.CRDTOperation, 0, 1024),
		maxLogSize:   10000,
		local:        NewLocalCollection(name, store),
	}

	dc.setupMessageHandlers()
	return dc
}

func (dc *DistributedCollection) setupMessageHandlers() {
	dc.network.OnMessage(typ.MsgOperation, func(msg typ.ProtocolMessage) {
		// Basic payload validation
		payload, ok := msg.Payload.(map[string]interface{})
		if !ok {
			return
		}
		coll, _ := payload["collection"].(string)
		if coll != dc.Name {
			return
		}
		opMap, _ := payload["operation"].(map[string]interface{})
		// We assume op was encoded in a friendly form; in a real implementation we'd use typed marshaling
		var op typ.CRDTOperation
		// re-marshal map into op
		b, _ := jsonMarshal(opMap)
		_ = jsonUnmarshal(b, &op)
		go dc.handleRemoteOperation(op)
	})

	dc.network.OnMessage(typ.MsgSyncRequest, func(msg typ.ProtocolMessage) {
		payload, _ := msg.Payload.(map[string]interface{})
		coll, _ := payload["collection"].(string)
		if coll != dc.Name {
			return
		}
		go dc.handleSyncRequest(msg)
	})

	dc.network.OnMessage(typ.MsgSyncResponse, func(msg typ.ProtocolMessage) {
		payload, _ := msg.Payload.(map[string]interface{})
		coll, _ := payload["collection"].(string)
		if coll != dc.Name {
			return
		}
		go dc.handleSyncResponse(msg)
	})
}

func (dc *DistributedCollection) AttachToNetwork(networkID string) error {
	if dc.networkID != "" {
		return fmt.Errorf("collection %s already attached to %s", dc.Name, dc.networkID)
	}

	if err := dc.network.AddCollectionToNetwork(networkID, dc.Name); err != nil {
		return err
	}

	dc.networkID = networkID
	dc.syncStates[networkID] = &typ.SyncState{Collection: dc.Name, NetworkID: networkID, LocalVector: clock.NewVectorClock(), LastSync: time.Now(), PendingOperations: nil, SyncInProgress: false}

	// Request initial sync
	return dc.requestSync()
}

func (dc *DistributedCollection) DetachFromNetwork() error {
	if dc.networkID == "" {
		return nil
	}
	_ = dc.network.RemoveCollectionFromNetwork(dc.networkID, dc.Name)
	delete(dc.syncStates, dc.networkID)
	dc.networkID = ""
	return nil
}

func (dc *DistributedCollection) Insert(ctx context.Context, doc map[string]interface{}) (map[string]interface{}, error) {
	id, ok := doc["id"].(string)
	if !ok {
		return nil, errors.New("document must contain 'id'")
	}
	entryType, _ := doc["entryType"].(typ.EntryType) // Assuming validation already happened

	// For MEMORY entries, blob is handled by storage
	if entryType == typ.EntryTypeMemory {
		if payload, ok := doc["payload"].(map[string]interface{}); ok {
			if _, hasBlob := payload["blob"]; hasBlob {
				// Blob will be saved by storage.Insert
			}
		}
	}

	// The local collection handles persistence. The underlying adapter is responsible
	// for the actual file I/O for blobs.
	inserted, err := dc.local.Insert(ctx, doc)
	if err != nil {
		return nil, err
	}

	if dc.networkID != "" {
		opPayload := resolver.ToDistributed(inserted, dc.network.GetPeerID())
		opPayload.EntryType = entryType // Ensure EntryType is set on the distributed doc

		op := typ.CRDTOperation{
			ID:         fmt.Sprintf("%s-%d-%d", dc.network.GetPeerID(), time.Now().UnixMilli(), time.Now().UnixNano()%1000),
			Type:       typ.OpInsert,
			Collection: dc.Name,
			DocumentID: id,
			Data:       opPayload,
			Vector:     dc.getCurrentVector(),
			Timestamp:  time.Now().UnixMilli(),
			PeerID:     dc.network.GetPeerID(),
		}
		dc.broadcastOperation(op)
	}

	return inserted, nil
}

func (dc *DistributedCollection) Update(id string, update map[string]interface{}) (int, error) {
	affected, err := dc.local.Update(id, update)
	if err != nil {
		return 0, err
	}
	if dc.networkID != "" && affected > 0 {
		doc, _ := dc.local.Find(id)
		op := typ.CRDTOperation{ID: fmt.Sprintf("%s-%d", dc.network.GetPeerID(), time.Now().UnixMilli()), Type: typ.OpUpdate, Collection: dc.Name, DocumentID: id, Data: resolver.ToDistributed(doc, dc.network.GetPeerID()), Vector: dc.getCurrentVector(), Timestamp: time.Now().UnixMilli(), PeerID: dc.network.GetPeerID()}
		dc.broadcastOperation(op)
	}
	return affected, nil
}

func (dc *DistributedCollection) Delete(id string) (int, error) {
	affected, err := dc.local.Delete(id)
	if err != nil {
		return 0, err
	}
	if dc.networkID != "" && affected > 0 {
		op := typ.CRDTOperation{ID: fmt.Sprintf("%s-%d", dc.network.GetPeerID(), time.Now().UnixMilli()), Type: typ.OpDelete, Collection: dc.Name, DocumentID: id, Data: &typ.DistributedDocument{ID: id, Deleted: true}, Vector: dc.getCurrentVector(), Timestamp: time.Now().UnixMilli(), PeerID: dc.network.GetPeerID()}
		dc.broadcastOperation(op)
	}
	return affected, nil
}

func (dc *DistributedCollection) Find(id string) (map[string]interface{}, error) {
	return dc.local.Find(id)
}
func (dc *DistributedCollection) FindAll() ([]map[string]interface{}, error) {
	return dc.local.store.FindAll(dc.Name)
}

func (dc *DistributedCollection) GetSyncState() *typ.SyncState {
	if dc.networkID == "" {
		return nil
	}
	return dc.syncStates[dc.networkID]
}

func (dc *DistributedCollection) ForceSync() error { return dc.requestSync() }

// Private helpers

func (dc *DistributedCollection) broadcastOperation(op typ.CRDTOperation) {
	if dc.networkID == "" {
		return
	}

	dc.mu.Lock()
	dc.operationLog = append(dc.operationLog, op)
	dc.pruneOperationLog()
	// increment local vector
	syncState := dc.syncStates[dc.networkID]
	syncState.LocalVector = clock.Increment(syncState.LocalVector, dc.network.GetPeerID())
	dc.mu.Unlock()

	_ = dc.network.BroadcastMessage(dc.networkID, typ.ProtocolMessage{Type: typ.MsgOperation, NetworkID: dc.networkID, SenderID: dc.network.GetPeerID(), Timestamp: time.Now().UnixMilli(), Payload: map[string]interface{}{"collection": dc.Name, "operation": op}})
}

func (dc *DistributedCollection) handleRemoteOperation(op typ.CRDTOperation) {
	// Apply CRDT operation to local document
	existing, _ := dc.local.Find(op.DocumentID)
	var existingDist *typ.DistributedDocument
	if existing != nil {
		existingDist = resolver.ToDistributed(existing, op.PeerID)
	}

	result := resolver.ApplyOperation(existingDist, op)

	if result == nil {
		// delete
		_, _ = dc.local.Delete(op.DocumentID)
	} else if result.Deleted {
		_, _ = dc.local.Delete(op.DocumentID)
	} else {
		// upsert
		_, _ = dc.local.Insert(context.Background(), resolver.ToRegular(result))
	}

	// merge vector
	if dc.networkID != "" {
		syncState := dc.syncStates[dc.networkID]
		syncState.LocalVector = clock.Merge(syncState.LocalVector, op.Vector)
	}
}

func (dc *DistributedCollection) requestSync() error {
	if dc.networkID == "" {
		return errors.New("not attached to network")
	}
	syncState := dc.syncStates[dc.networkID]
	if syncState.SyncInProgress {
		return nil
	}
	syncState.SyncInProgress = true

	_ = dc.network.BroadcastMessage(dc.networkID, typ.ProtocolMessage{Type: typ.MsgSyncRequest, NetworkID: dc.networkID, SenderID: dc.network.GetPeerID(), Timestamp: time.Now().UnixMilli(), Payload: map[string]interface{}{"collection": dc.Name, "vector": syncState.LocalVector}})

	// Clear flag after timeout
	go func() {
		time.Sleep(10 * time.Second)
		syncState.SyncInProgress = false
	}()

	return nil
}

func (dc *DistributedCollection) handleSyncRequest(msg typ.ProtocolMessage) {
	payload, _ := msg.Payload.(map[string]interface{})
	remoteVector, _ := payload["vector"].(map[string]interface{})

	// convert remoteVector into VectorClock
	rv := make(clock.VectorClock)
	for k, v := range remoteVector {
		switch val := v.(type) {
		case float64:
			rv[k] = int64(val)
		case int64:
			rv[k] = val
		}
	}

	// find missing ops
	missing := make([]typ.CRDTOperation, 0)
	for _, op := range dc.operationLog {
		remoteClock := int64(0)
		if vv, ok := rv[op.PeerID]; ok {
			remoteClock = vv
		}
		opClock := int64(0)
		if vv, ok := op.Vector[op.PeerID]; ok {
			opClock = vv
		}
		if opClock > remoteClock {
			missing = append(missing, op)
		}
	}

	_ = dc.network.SendToPeer(msg.SenderID, dc.networkID, typ.ProtocolMessage{Type: typ.MsgSyncResponse, NetworkID: dc.networkID, SenderID: dc.network.GetPeerID(), Timestamp: time.Now().UnixMilli(), Payload: map[string]interface{}{"collection": dc.Name, "operations": missing, "vector": dc.syncStates[dc.networkID].LocalVector}})
}

func (dc *DistributedCollection) handleSyncResponse(msg typ.ProtocolMessage) {
	payload, _ := msg.Payload.(map[string]interface{})
	opsIface, _ := payload["operations"].([]interface{})

	for _, oi := range opsIface {
		b, _ := jsonMarshal(oi)
		var op typ.CRDTOperation
		_ = jsonUnmarshal(b, &op)
		dc.handleRemoteOperation(op)
	}

	if dc.networkID != "" {
		syncState := dc.syncStates[dc.networkID]
		syncState.SyncInProgress = false
		syncState.LastSync = time.Now()
	}
}

func (dc *DistributedCollection) getCurrentVector() clock.VectorClock {
	if dc.networkID == "" {
		return clock.NewVectorClock()
	}
	if s, ok := dc.syncStates[dc.networkID]; ok {
		return clock.Clone(s.LocalVector)
	}
	return clock.NewVectorClock()
}

func (dc *DistributedCollection) pruneOperationLog() {
	if len(dc.operationLog) > dc.maxLogSize {
		dc.operationLog = dc.operationLog[len(dc.operationLog)-dc.maxLogSize:]
	}
}

// Helper JSON marshalling/unmarshalling helpers used by the example to avoid importing encoding/json repeatedly
func jsonMarshal(v interface{}) ([]byte, error)   { return json.Marshal(v) }
func jsonUnmarshal(b []byte, v interface{}) error { return json.Unmarshal(b, &v) }
