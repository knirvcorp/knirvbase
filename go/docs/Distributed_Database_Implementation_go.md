# NebulusDB Distributed Database Implementation (Golang)

## Overview

This document outlines a Golang translation of the NebulusDB distributed database implementation. It maps the TypeScript implementation to idiomatic Go (types, methods, and tests) while keeping the same architecture: DHT-based peer-to-peer synchronization with CRDT-based conflict resolution.

## Architecture Goals

- **Local-First:** All operations are local-first with background sync.
- **Network Isolation:** Each peer network is an independent consortium with its own collections.
- **Dynamic Membership:** Collections can be added/removed at runtime.
- **Conflict Resolution:** CRDT-based automatic conflict resolution with vector clocks.
- **Type Safety:** Static Go types across the implementation.
- **Backward Compatible:** Existing non-distributed code paths can remain unchanged.

## Storage and Persistence

The KNIRV network utilizes a custom-built storage and persistence layer designed for flexibility and efficiency in a distributed environment. This layer implements the storage methods detailed in this document, with optimized strategies for both `MEMORY` and `AUTH` data entry types.

### Key Storage Features:

*   **Default Authentication:** Authentication handling is included by default to ensure secure access to the database. All `AUTH` type entries are stored using a simple, encrypted key-value model.
*   **File Storage Location:** All database files are stored in the standard OS application data directory (e.g., `~/.config` on Linux, `%APPDATA%` on Windows) to ensure proper separation and user-specific data management.
*   **`MEMORY` Entry Type Storage:** Entries of the `MEMORY` type are persisted using a multi-file structure on the **localhost filesystem** within the OS application data directory. This separation allows for efficient handling of large binary data while keeping metadata and vectors easily accessible. An IPFS-based storage solution for distributed blob storage is planned for a future release.

    For each `MEMORY` entry, the following are saved:
    1.  **Data Blob:** The raw data is saved as a separate file on the local system. 
    2.  **Vector Representation:** The vector embedding is stored alongside the metadata within the main database file/store. It is synchronized between peers.
    3.  **Metadata Facts Table:** The structured metadata is stored in the main database file/store and is synchronized between peers.

    The synchronized document (`DistributedDocument`) contains metadata about the blob (like its path or a content hash), but the blob data itself is **not** synchronized across the network in this version.


### Design Rationale: Why Blobs Are Not Synchronized

In this architecture, only the metadata and vector representation for `MEMORY` entries are synchronized across peers. The raw data blob is stored only on the local system where it was created. This design was chosen for several key reasons related to performance and efficiency in a distributed system:

1.  **Network Efficiency:** Broadcasting large binary files (blobs) to every peer upon every change would consume significant bandwidth, leading to slow synchronization and a less responsive network.
2.  **Storage Overhead:** Requiring every peer to store a copy of every blob from all other peers would lead to excessive storage consumption. This design allows peers to only store the blobs they explicitly need.
3.  **On-Demand Fetching:** The synchronized metadata includes a reference to the blob (like a path or content hash). A peer that needs the full blob can use this reference to request it directly from the origin peer or a future dedicated storage layer (like IPFS). This pattern of "synchronizing discovery, not data" is a standard practice for keeping distributed systems fast and scalable.


### KNIRVQL: The KNIRV Query Language

To facilitate easy interaction with the database, the system will feature a custom query language called **KNIRVQL**. It is designed to be a simple, human-readable language for performing CRUD operations and specialized queries on both `MEMORY` and `AUTH` entry types.

**Core Features of KNIRVQL:**

*   **Unified Interface:** A single language to manage authentication keys (`AUTH`) and complex data (`MEMORY`).
*   **Metadata Filtering:** Powerful filtering capabilities on the metadata facts table of `MEMORY` entries.
*   **Vector Search:** Native support for vector similarity searches on `MEMORY` entries.

**Example Queries:**

```knirvql
// Find the 10 most similar memory entries based on a vector, filtered by metadata
GET MEMORY WHERE metadata.source = "web-scrape" SIMILAR TO [0.45, 0.12, ...] LIMIT 10;

// Create or update an authentication key
SET AUTH key = "google_maps_api_key" value = "AIzaSy...";

// Retrieve an authentication key
GET AUTH WHERE key = "google_maps_api_key";

// Delete an entry by its ID
DELETE WHERE id = "entry-id-12345";
```


## Technology Stack

- **DHT/P2P Layer:** go-libp2p (production-grade P2P networking)
- **Conflict Resolution:** CRDT (vector clocks)
- **Transport:** TCP, WebRTC (via libp2p transports)
- **Discovery:** mDNS (local) and bootstrap peers (remote)
- **Encryption:** noise/TLS via go-libp2p

---

## Core Components

### 1) Network Types and Interfaces (types.go)

```go
package distributed

import (
    "time"

    ma "github.com/multiformats/go-multiaddr"
)

// VectorClock maps peer IDs to counters
type VectorClock map[string]int64

// EntryType specifies the kind of data stored.
type EntryType string

const (
    EntryTypeMemory EntryType = "MEMORY"
    EntryTypeAuth   EntryType = "AUTH"
)

// DistributedDocument augments a document with CRDT metadata
type DistributedDocument struct {
    ID        string                 `json:"id"`
    EntryType EntryType              `json:"entryType"`
    Payload   map[string]interface{} `json:"payload,omitempty"`
    Vector    VectorClock            `json:"_vector"`
    Timestamp int64                  `json:"_timestamp"`
    PeerID    string                 `json:"_peerId"`
    // Stage is an optional marker used to indicate special handling for a document.
    // Supported values: "post-pending" (document is staged and will be posted as a KNIRVGRAPH
    // transaction during the next sync), or empty string for normal documents.
    Stage     string                 `json:"_stage,omitempty"`
    Deleted   bool                   `json:"_deleted,omitempty"`
}

// OperationType enumerates CRDT operation kinds
type OperationType int

const (
    OpInsert OperationType = iota
    OpUpdate
    OpDelete
)

// CRDTOperation represents a change to be synchronized
type CRDTOperation struct {
    ID         string      `json:"id"`
    Type       OperationType `json:"type"`
    Collection string      `json:"collection"`
    DocumentID string      `json:"documentId"`
    Data       *DistributedDocument `json:"data,omitempty"`
    Vector     VectorClock `json:"vector"`
    Timestamp  int64       `json:"timestamp"`
    PeerID     string      `json:"peerId"`
}

// NetworkConfig holds network-level configuration
// Additional fields control posting/staging behavior:
//  - DefaultPostingNetwork: the network to which staged entries are posted (e.g. "knirvgraph").
//  - AutoPostClassifications: a list of EntryTypes that are automatically staged for posting by classification.
//  - PrivateByDefault: when true (default), entries are private unless staged or explicitly configured.
type NetworkConfig struct {
    NetworkID      string
    Name           string
    Collections    map[string]bool
    BootstrapPeers []ma.Multiaddr
    // Default posting target for staged entries (e.g., "knirvgraph").
    DefaultPostingNetwork string

    // Entry classifications which are auto-staged for posting. Common defaults include
    // EntryType values like "ERROR", "CONTEXT", and "IDEA".
    AutoPostClassifications []EntryType

    // Entries are private by default unless staged or configured otherwise.
    PrivateByDefault bool

    Encryption     struct {
        Enabled      bool
        SharedSecret string
    }
    Replication struct {
        Factor   int
        Strategy string // full | partial | leader
    }
    Discovery struct {
        MDNS      bool
        Bootstrap bool
    }
}

// PeerInfo
type PeerInfo struct {
    PeerID    string
    Addrs     []ma.Multiaddr
    Protocols []string
    Latency   time.Duration
    LastSeen  time.Time
    Collections []string
}

// SyncState for a collection/network
type SyncState struct {
    Collection        string
    NetworkID         string
    LocalVector       VectorClock
    LastSync          time.Time
    PendingOperations []CRDTOperation
    // StagedEntries contains IDs of documents marked with `_stage == "post-pending"`.
    // These will be converted to KNIRVGRAPH transactions and posted during the next sync.
    StagedEntries     []string
    SyncInProgress    bool
}

// NetworkStats
type NetworkStats struct {
    NetworkID         string
    ConnectedPeers    int
    TotalPeers        int
    CollectionsShared int
    OperationsSent    int64
    OperationsReceived int64
    BytesTransferred  int64
    AverageLatency    time.Duration
}

// MessageType strings for protocol
type MessageType string

const (
    MsgSyncRequest      MessageType = "sync_request"
    MsgSyncResponse     MessageType = "sync_response"
    MsgOperation        MessageType = "operation"
    MsgHeartbeat        MessageType = "heartbeat"
    MsgCollectionAnnounce MessageType = "collection_announce"
    MsgCollectionRequest MessageType = "collection_request"
)

// ProtocolMessage generic envelope
type ProtocolMessage struct {
    Type      MessageType    `json:"type"`
    NetworkID string         `json:"networkId"`
    SenderID  string         `json:"senderId"`
    Timestamp int64          `json:"timestamp"`
    Payload   interface{}    `json:"payload"`
}
```

---

### 2) Vector Clock Implementation (vector_clock.go)

## Post-Pending Staging

Only documents that are *staged* (`_stage == "post-pending"`) are synchronized with a chosen network. By default, entries classified as **Error**, **Context**, and **Idea** are **auto-staged** to the `KNIRVGRAPH` posting network (unless overridden by network configuration). All other entries are **private by default** and will not be synchronized or posted unless explicitly staged or configured with auto-post rules.

Behavior summary:

- **Only staged entries are synchronized with a network.** Staging is the explicit signal to convert a local document into a network-postable transaction.
- Staged (`_stage == "post-pending"`) documents remain local and are not broadcast as standard CRDT operations; they are processed by the sync posting flow instead.
- The `SyncState.StagedEntries` list records IDs of documents with `post-pending` stage for each collection/network.
- The `NetworkConfig` can be used to control default posting behavior (examples below):
  - `DefaultPostingNetwork` (e.g. `"knirvgraph"`)
  - `AutoPostClassifications` ([]EntryType) — classifications to auto-stage
  - `PrivateByDefault` (bool) — whether non-classified entries remain private

- During the next successful sync for the collection, the sync routine will:
  1. Collect staged entries from `StagedEntries`.
  2. Convert each staged `DistributedDocument` into a KNIRVGRAPH transaction (payload + metadata), or into a transaction for the configured `DefaultPostingNetwork`.
  3. Submit the transaction to the configured graph/client (async/queued as appropriate).
  4. On successful submission, clear the document's `_stage` field and remove its ID from `StagedEntries`.

Example (pseudocode):

```go
// Stage a document for posting
func (db *DB) StageForPost(collection, docID string) error {
    doc := db.Get(collection, docID)
    if doc == nil { return fmt.Errorf("not found") }
    doc.Stage = "post-pending"
    db.Save(collection, doc)
    s := db.syncState(collection)
    s.StagedEntries = append(s.StagedEntries, docID)
    db.saveSyncState(s)
    return nil
}

// Auto-stage on save when classification matches network config
func (db *DB) Save(collection string, doc *DistributedDocument) error {
    if db.networkConfig.AutoPostClassifications != nil {
        for _, c := range db.networkConfig.AutoPostClassifications {
            if doc.Payload["classification"] == string(c) {
                doc.Stage = "post-pending"
                // ensure staged list keeps in sync
                s := db.syncState(collection)
                s.StagedEntries = append(s.StagedEntries, doc.ID)
                db.saveSyncState(s)
                break
            }
        }
    }
    // regular persistence + CRDT handling
    return db.persist(document)
}

// During sync
func (s *SyncManager) handleSync(networkID string) {
    staged := s.getStagedEntries(networkID)
    for _, docID := range staged {
        doc := s.db.Get(s.collection, docID)
        if doc == nil { continue }
        if doc.Stage != "post-pending" { continue }

        tx := buildKNIRVGraphTransaction(doc)
        if err := s.knirvClient.SubmitTransaction(tx); err != nil {
            // keep staged for retry
            continue
        }

        // on success, clear stage and update store
        doc.Stage = ""
        s.db.Save(s.collection, doc)
        s.removeStagedEntry(networkID, docID)
    }
}
```

Note: posting to KNIRVGRAPH is an orthogonal operation to CRDT synchronization; CRDT operations remain the authoritative mechanism for syncing data state, while `post-pending` provides a way to publish selected entries as on-chain/graph transactions during scheduled syncs.



```go
package distributed

import "math"

// ComparisonResult is the relationship between two vector clocks
type ComparisonResult int

const (
    Equal ComparisonResult = iota
    Before
    After
    Concurrent
)

// Increment increments a peer counter on the vector clock
func Increment(clock VectorClock, peerID string) VectorClock {
    if clock == nil {
        clock = make(VectorClock)
    }
    clock[peerID] = clock[peerID] + 1
    return clock
}

// Merge two vector clocks (take max per peer)
func Merge(clock1, clock2 VectorClock) VectorClock {
    merged := make(VectorClock)
    for k, v := range clock1 {
        merged[k] = v
    }
    for k, v := range clock2 {
        if existing, ok := merged[k]; !ok || v > existing {
            merged[k] = v
        }
    }
    return merged
}

// Compare returns Equal|Before|After|Concurrent
func Compare(clock1, clock2 VectorClock) ComparisonResult {
    hasGreater, hasLess := false, false

    allKeys := make(map[string]struct{})
    for k := range clock1 { allKeys[k] = struct{}{} }
    for k := range clock2 { allKeys[k] = struct{}{} }

    for k := range allKeys {
        v1 := int64(0)
        v2 := int64(0)
        if vv, ok := clock1[k]; ok { v1 = vv }
        if vv, ok := clock2[k]; ok { v2 = vv }

        if v1 > v2 { hasGreater = true }
        if v1 < v2 { hasLess = true }
    }

    switch {
    case !hasGreater && !hasLess:
        return Equal
    case hasGreater && !hasLess:
        return After
    case hasLess && !hasGreater:
        return Before
    default:
        return Concurrent
    }
}

// HappensBefore returns true if clock1 is before or equal to clock2
func HappensBefore(clock1, clock2 VectorClock) bool {
    return Compare(clock1, clock2) == Before || Compare(clock1, clock2) == Equal
}

// NewVectorClock returns an empty clock
func NewVectorClock() VectorClock { return make(VectorClock) }

// Clone returns a shallow copy
func Clone(clock VectorClock) VectorClock {
    if clock == nil { return nil }
    copy := make(VectorClock, len(clock))
    for k, v := range clock { copy[k] = v }
    return copy
}
```

---

### 3) CRDT Conflict Resolution (crdt_resolver.go)

```go
package distributed

import "time"

// ResolveConflict applies LWW + vector-clock tie-breaking
func ResolveConflict(local, remote *DistributedDocument) *DistributedDocument {
    if remote == nil { return local }
    if local == nil { return remote }

    // Deletion handling
    if remote.Deleted && !local.Deleted {
        comp := Compare(local.Vector, remote.Vector)
        if comp == Before || comp == Concurrent {
            return remote
        }
        return local
    }
    if local.Deleted && !remote.Deleted {
        comp := Compare(remote.Vector, local.Vector)
        if comp == Before || comp == Concurrent {
            return local
        }
        return remote
    }

    switch Compare(local.Vector, remote.Vector) {
    case After:
        return local
    case Before:
        return remote
    case Equal:
        return local
    case Concurrent:
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

func mergeDocuments(winner, loser *DistributedDocument) *DistributedDocument {
    merged := *winner // shallow copy
    merged.Vector = Merge(winner.Vector, loser.Vector)

    // Merge non-conflicting fields from loser payload
    if merged.Payload == nil { merged.Payload = make(map[string]interface{}) }
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
func ApplyOperation(doc *DistributedDocument, op CRDTOperation) *DistributedDocument {
    switch op.Type {
    case OpInsert, OpUpdate:
        if doc == nil {
            if op.Data == nil { return nil }
            copy := *op.Data
            copy.Vector = Clone(op.Vector)
            copy.Timestamp = op.Timestamp
            copy.PeerID = op.PeerID
            return &copy
        }

        comp := Compare(doc.Vector, op.Vector)
        if comp == Before || comp == Concurrent {
            // Merge fields
            if doc.Payload == nil { doc.Payload = make(map[string]interface{}) }
            for k, v := range op.Data.Payload {
                doc.Payload[k] = v
            }
            doc.Vector = Merge(doc.Vector, op.Vector)
            if op.Timestamp > doc.Timestamp { doc.Timestamp = op.Timestamp }
        }
        return doc

    case OpDelete:
        if doc == nil { return nil }
        comp := Compare(doc.Vector, op.Vector)
        if comp == Before || comp == Concurrent {
            doc.Deleted = true
            doc.Vector = Merge(doc.Vector, op.Vector)
            if op.Timestamp > doc.Timestamp { doc.Timestamp = op.Timestamp }
        }
        return doc
    default:
        return doc
    }
}

// ToDistributed converts a simple map payload to DistributedDocument
func ToDistributed(payload map[string]interface{}, peerID string) *DistributedDocument {
    v := NewVectorClock()
    v[peerID] = 1
    return &DistributedDocument{
        ID:        payload["id"].(string),
        Payload:   payload,
        Vector:    v,
        Timestamp: time.Now().UnixMilli(),
        PeerID:    peerID,
    }
}

// ToRegular converts a DistributedDocument to a regular map representation
func ToRegular(doc *DistributedDocument) map[string]interface{} {
    if doc == nil { return nil }
    out := make(map[string]interface{})
    for k, v := range doc.Payload { out[k] = v }
    return out
}
```

---

### 4) Network Manager (network_manager.go)

```go
package distributed

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "sync"
    "time"

    libp2p "github.com/libp2p/go-libp2p"
    dht "github.com/libp2p/go-libp2p-kad-dht"
    mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
    discovery "github.com/libp2p/go-libp2p/p2p/discovery"
    noise "github.com/libp2p/go-libp2p-noise"
    golog "github.com/ipfs/go-log/v2"
    host "github.com/libp2p/go-libp2p/core/host"
    network "github.com/libp2p/go-libp2p/core/network"
    peer "github.com/libp2p/go-libp2p/core/peer"
    ma "github.com/multiformats/go-multiaddr"
)

var log = golog.Logger("nebula:network")

// MessageHandler receives a ProtocolMessage
type MessageHandler func(msg ProtocolMessage)

// Network defines the behaviour used by the distributed components. It enables tests to use a mock implementation.
type Network interface {
    Initialize() error
    CreateNetwork(cfg NetworkConfig) (string, error)
    JoinNetwork(networkID string, bootstrapPeers []string) error
    LeaveNetwork(networkID string) error

    AddCollectionToNetwork(networkID, collectionName string) error
    RemoveCollectionFromNetwork(networkID, collectionName string) error
    GetNetworkCollections(networkID string) []string

    BroadcastMessage(networkID string, msg ProtocolMessage) error
    SendToPeer(peerID string, networkID string, msg ProtocolMessage) error
    OnMessage(mt MessageType, handler MessageHandler)

    GetNetworkStats(networkID string) *NetworkStats
    GetNetworks() []*NetworkConfig
    GetPeerID() string
    Shutdown() error
}

// NetworkManager is a concrete implementation using go-libp2p
type NetworkManager struct {
    ctx       context.Context
    cancel    context.CancelFunc
    host      host.Host
    dht       *dht.IpfsDHT
    mdns      discovery.Service

    mu           sync.RWMutex
    networks     map[string]*NetworkConfig
    peers        map[string]*PeerInfo
    stats        map[string]*NetworkStats
    handlers     map[MessageType][]MessageHandler
    initialized  bool
    peerID       string
}

func NewNetworkManager(ctx context.Context) *NetworkManager {
    c, cancel := context.WithCancel(ctx)
    return &NetworkManager{
        ctx:      c,
        cancel:   cancel,
        networks: make(map[string]*NetworkConfig),
        peers:    make(map[string]*PeerInfo),
        stats:    make(map[string]*NetworkStats),
        handlers: make(map[MessageType][]MessageHandler),
    }
}

func (n *NetworkManager) Initialize() error {
    n.mu.Lock()
    defer n.mu.Unlock()
    if n.initialized { return nil }

    // Create libp2p host with noise security and default transports
    h, err := libp2p.New(
        libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
        libp2p.Security(noise.ID, noise.New),
    )
    if err != nil {
        return err
    }

    // Create DHT (best-effort)
    d, err := dht.New(n.ctx, h)
    if err != nil {
        // Non-fatal in some environments; keep d nil and log
        log.Infof("failed to create DHT: %v", err)
    } else {
        n.dht = d
    }

    // Setup mDNS discovery for local peers
    mdnsSvc, err := mdns.NewMdnsService(n.ctx, h, time.Second*5, "nebula-mdns")
    if err != nil {
        log.Warnf("failed to start mDNS: %v", err)
    } else {
        n.mdns = mdnsSvc
        mdnsSvc.RegisterNotifee(&mdnsNotifee{nm: n})
    }

    // Register network notifier for connect/disconnect events
    h.Network().Notify(&networkNotifee{nm: n})

    n.host = h
    n.peerID = h.ID().Pretty()
    n.initialized = true

    log.Infof("node initialized: %s", n.peerID)
    return nil
}

func (n *NetworkManager) CreateNetwork(cfg NetworkConfig) (string, error) {
    if err := n.Initialize(); err != nil { return "", err }

    n.mu.Lock()
    defer n.mu.Unlock()

    if _, exists := n.networks[cfg.NetworkID]; exists {
        return cfg.NetworkID, nil
    }

    cfg.Collections = make(map[string]bool)
    n.networks[cfg.NetworkID] = &cfg
    n.stats[cfg.NetworkID] = &NetworkStats{NetworkID: cfg.NetworkID}

    protocolID := fmt.Sprintf("/nebulusdb/%s/1.0.0", cfg.NetworkID)

    // Register a stream handler; handler is lightweight and delegates to a goroutine
    n.host.SetStreamHandler(protocolID, func(s network.Stream) {
        go n.handleIncomingStream(s, cfg.NetworkID)
    })

    n.mu.Unlock()
    log.Infof("created network %s", cfg.NetworkID)
    n.mu.Lock()

    return cfg.NetworkID, nil
}

func (n *NetworkManager) JoinNetwork(networkID string, bootstrapPeers []string) error {
    if err := n.Initialize(); err != nil { return err }

    n.mu.Lock()
    if _, exists := n.networks[networkID]; !exists {
        n.networks[networkID] = &NetworkConfig{NetworkID: networkID, Name: fmt.Sprintf("Network %s", networkID), Collections: make(map[string]bool)}
        n.stats[networkID] = &NetworkStats{NetworkID: networkID}
    }
    n.mu.Unlock()

    // Attempt to connect to bootstrap peers
    for _, addr := range bootstrapPeers {
        maddr, err := ma.NewMultiaddr(addr)
        if err != nil {
            log.Warnf("invalid bootstrap address %s: %v", addr, err)
            continue
        }

        info, err := peer.AddrInfoFromP2pAddr(maddr)
        if err != nil {
            log.Warnf("failed to get peer info from %s: %v", addr, err)
            continue
        }

        ctx, cancel := context.WithTimeout(n.ctx, 5*time.Second)
        err = n.host.Connect(ctx, *info)
        cancel()
        if err != nil {
            log.Warnf("failed to connect to bootstrap %s: %v", addr, err)
            continue
        }
        log.Infof("connected to bootstrap peer %s", info.ID.Pretty())
    }

    return nil
}

func (n *NetworkManager) LeaveNetwork(networkID string) error {
    n.mu.Lock()
    defer n.mu.Unlock()

    if _, ok := n.networks[networkID]; !ok { return nil }

    // Unregister handler by setting a no-op handler to immediately close streams
    protocolID := fmt.Sprintf("/nebulusdb/%s/1.0.0", networkID)
    n.host.SetStreamHandler(protocolID, func(s network.Stream) { _ = s.Close() })

    // Remove the config
    delete(n.networks, networkID)
    delete(n.stats, networkID)

    log.Infof("left network %s", networkID)
    return nil
}

func (n *NetworkManager) AddCollectionToNetwork(networkID, collectionName string) error {
    n.mu.Lock()
    netCfg, ok := n.networks[networkID]
    n.mu.Unlock()
    if !ok { return errors.New("network not found") }

    n.mu.Lock()
    netCfg.Collections[collectionName] = true
    if st, ok := n.stats[networkID]; ok {
        st.CollectionsShared = len(netCfg.Collections)
    }
    n.mu.Unlock()

    // Announce collection
    return n.BroadcastMessage(networkID, ProtocolMessage{
        Type: MsgCollectionAnnounce,
        NetworkID: networkID,
        SenderID: n.GetPeerID(),
        Timestamp: time.Now().UnixMilli(),
        Payload: map[string]string{"collection": collectionName},
    })
}

func (n *NetworkManager) RemoveCollectionFromNetwork(networkID, collectionName string) error {
    n.mu.Lock()
    defer n.mu.Unlock()
    netCfg, ok := n.networks[networkID]
    if !ok { return nil }
    delete(netCfg.Collections, collectionName)
    if st, ok := n.stats[networkID]; ok {
        st.CollectionsShared = len(netCfg.Collections)
    }
    return nil
}

func (n *NetworkManager) GetNetworkCollections(networkID string) []string {
    n.mu.RLock()
    defer n.mu.RUnlock()
    netCfg, ok := n.networks[networkID]
    if !ok { return nil }
    out := make([]string, 0, len(netCfg.Collections))
    for c := range netCfg.Collections { out = append(out, c) }
    return out
}

func (n *NetworkManager) BroadcastMessage(networkID string, msg ProtocolMessage) error {
    if !n.initialized { return errors.New("not initialized") }

    protocolID := fmt.Sprintf("/nebulusdb/%s/1.0.0", networkID)
    data, err := json.Marshal(msg)
    if err != nil { return err }

    peers := n.host.Network().Peers()
    st := n.GetNetworkStats(networkID)

    for _, p := range peers {
        if p == n.host.ID() { continue }
        ctx, cancel := context.WithTimeout(n.ctx, 5*time.Second)
        stream, err := n.host.NewStream(ctx, p, protocolID)
        cancel()
        if err != nil {
            log.Warnf("failed to open stream to %s: %v", p.Pretty(), err)
            continue
        }

        _, err = stream.Write(append(data, '\n'))
        _ = stream.Close()
        if err != nil {
            log.Warnf("failed to write to stream %s: %v", p.Pretty(), err)
            continue
        }

        if st != nil { st.OperationsSent++; st.BytesTransferred += int64(len(data)) }
    }

    return nil
}

func (n *NetworkManager) SendToPeer(peerID string, networkID string, msg ProtocolMessage) error {
    if !n.initialized { return errors.New("not initialized") }

    pid, err := peer.Decode(peerID)
    if err != nil { return err }

    protocolID := fmt.Sprintf("/nebulusdb/%s/1.0.0", networkID)

    ctx, cancel := context.WithTimeout(n.ctx, 5*time.Second)
    stream, err := n.host.NewStream(ctx, pid, protocolID)
    cancel()
    if err != nil { return err }

    data, err := json.Marshal(msg)
    if err != nil { _ = stream.Close(); return err }

    _, err = stream.Write(append(data, '\n'))
    _ = stream.Close()
    if err != nil { return err }

    if st := n.GetNetworkStats(networkID); st != nil { st.OperationsSent++; st.BytesTransferred += int64(len(data)) }

    return nil
}

func (n *NetworkManager) OnMessage(mt MessageType, handler MessageHandler) {
    n.mu.Lock()
    defer n.mu.Unlock()
    n.handlers[mt] = append(n.handlers[mt], handler)
}

func (n *NetworkManager) GetNetworkStats(networkID string) *NetworkStats {
    n.mu.RLock()
    defer n.mu.RUnlock()
    st := n.stats[networkID]
    return st
}

func (n *NetworkManager) GetNetworks() []*NetworkConfig {
    n.mu.RLock()
    defer n.mu.RUnlock()
    out := make([]*NetworkConfig, 0, len(n.networks))
    for _, v := range n.networks { out = append(out, v) }
    return out
}

func (n *NetworkManager) GetPeerID() string { return n.peerID }

func (n *NetworkManager) Shutdown() error {
    n.cancel()
    if n.mdns != nil { _ = n.mdns.Close() }
    if n.dht != nil { _ = n.dht.Close() }
    if n.host != nil { _ = n.host.Close() }
    n.mu.Lock()
    n.initialized = false
    n.mu.Unlock()
    return nil
}

// Internal: handle incoming stream and decode ProtocolMessage lines
func (n *NetworkManager) handleIncomingStream(s network.Stream, networkID string) {
    defer s.Close()
    st := n.GetNetworkStats(networkID)

    dec := json.NewDecoder(s)
    for {
        var msg ProtocolMessage
        if err := dec.Decode(&msg); err != nil {
            if err == io.EOF { return }
            log.Warnf("failed to decode incoming message: %v", err)
            return
        }

        if st != nil { st.OperationsReceived++; st.BytesTransferred += int64(len(msg.Payload.(string))) }

        n.handleMessage(msg)
    }
}

func (n *NetworkManager) handleMessage(msg ProtocolMessage) {
    n.mu.RLock()
    handlers := n.handlers[msg.Type]
    n.mu.RUnlock()

    for _, h := range handlers {
        go func(fn MessageHandler) {
            defer func() { _ = recover() }()
            fn(msg)
        }(h)
    }
}

// mDNS notifee
type mdnsNotifee struct{ nm *NetworkManager }

func (m *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
    m.nm.mu.Lock()
    defer m.nm.mu.Unlock()
    pid := pi.ID.Pretty()
    if _, ok := m.nm.peers[pid]; !ok {
        m.nm.peers[pid] = &PeerInfo{
            PeerID: pid,
            Addrs: pi.Addrs,
            Protocols: []string{},
            LastSeen: time.Now(),
        }
    }
    // Optionally connect
    ctx, cancel := context.WithTimeout(m.nm.ctx, 5*time.Second)
    _ = m.nm.host.Connect(ctx, pi)
    cancel()
}

// network.Notify notifee
type networkNotifee struct{ nm *NetworkManager }

func (n *networkNotifee) Listen(network.Network, ma.Multiaddr)      {}
func (n *networkNotifee) ListenClose(network.Network, ma.Multiaddr) {}
func (n *networkNotifee) Connected(net network.Network, conn network.Conn) {
    pid := conn.RemotePeer().Pretty()
    n.nm.mu.Lock()
    if p, ok := n.nm.peers[pid]; ok {
        p.LastSeen = time.Now()
    } else {
        n.nm.peers[pid] = &PeerInfo{PeerID: pid, LastSeen: time.Now()}
    }

    for _, st := range n.nm.stats { st.ConnectedPeers = len(n.nm.host.Network().Peers()) }
    n.nm.mu.Unlock()
}
func (n *networkNotifee) Disconnected(net network.Network, conn network.Conn) {
    for _, st := range n.nm.stats { st.ConnectedPeers = len(n.nm.host.Network().Peers()) }
}
func (n *networkNotifee) OpenedStream(network.Network, network.Stream) {}
func (n *networkNotifee) ClosedStream(network.Network, network.Stream) {}
```

---

### 5) Distributed Collection (distributed_collection.go)

```go
package distributed

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "sync"
    "time"
)

// A minimal in-memory local collection implementation to keep the example self-contained
type LocalCollection struct {
    mu    sync.RWMutex
    name  string
    store map[string]map[string]interface{}
}

func NewLocalCollection(name string) *LocalCollection {
    return &LocalCollection{name: name, store: make(map[string]map[string]interface{})}
}

func (c *LocalCollection) Insert(doc map[string]interface{}) (map[string]interface{}, error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    id, ok := doc["id"].(string)
    if !ok || id == "" {
        return nil, errors.New("document must have string id")
    }
    c.store[id] = cloneMap(doc)
    return cloneMap(doc), nil
}

func (c *LocalCollection) Update(id string, update map[string]interface{}) (int, error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    d, ok := c.store[id]
    if !ok { return 0, nil }
    for k, v := range update { d[k] = v }
    return 1, nil
}

func (c *LocalCollection) Delete(id string) (int, error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    _, ok := c.store[id]
    if !ok { return 0, nil }
    delete(c.store, id)
    return 1, nil
}

func (c *LocalCollection) Find(id string) (map[string]interface{}, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if d, ok := c.store[id]; ok { return cloneMap(d), nil }
    return nil, nil
}

func cloneMap(m map[string]interface{}) map[string]interface{} {
    out := make(map[string]interface{}, len(m))
    for k, v := range m { out[k] = v }
    return out
}

// DistributedCollection manages local storage plus network synchronization
type DistributedCollection struct {
    Name          string
    network       Network
    networkID     string
    syncStates    map[string]*SyncState
    operationLog  []CRDTOperation
    maxLogSize    int

    local         *LocalCollection
    mu            sync.Mutex
}

func NewDistributedCollection(name string, net Network) *DistributedCollection {
    dc := &DistributedCollection{
        Name: name,
        network: net,
        syncStates: make(map[string]*SyncState),
        operationLog: make([]CRDTOperation, 0, 1024),
        maxLogSize: 10000,
        local: NewLocalCollection(name),
    }

    dc.setupMessageHandlers()
    return dc
}

func (dc *DistributedCollection) setupMessageHandlers() {
    dc.network.OnMessage(MsgOperation, func(msg ProtocolMessage) {
        // Basic payload validation
        payload, ok := msg.Payload.(map[string]interface{})
        if !ok { return }
        coll, _ := payload["collection"].(string)
        if coll != dc.Name { return }
        opMap, _ := payload["operation"].(map[string]interface{})
        // We assume op was encoded in a friendly form; in a real implementation we'd use typed marshaling
        var op CRDTOperation
        // re-marshal map into op
        b, _ := jsonMarshal(opMap)
        _ = jsonUnmarshal(b, &op)
        go dc.handleRemoteOperation(op)
    })

    dc.network.OnMessage(MsgSyncRequest, func(msg ProtocolMessage) {
        payload, _ := msg.Payload.(map[string]interface{})
        coll, _ := payload["collection"].(string)
        if coll != dc.Name { return }
        go dc.handleSyncRequest(msg)
    })

    dc.network.OnMessage(MsgSyncResponse, func(msg ProtocolMessage) {
        payload, _ := msg.Payload.(map[string]interface{})
        coll, _ := payload["collection"].(string)
        if coll != dc.Name { return }
        go dc.handleSyncResponse(msg)
    })
}

func (dc *DistributedCollection) AttachToNetwork(networkID string) error {
    if dc.networkID != "" { return fmt.Errorf("collection %s already attached to %s", dc.Name, dc.networkID) }

    if err := dc.network.AddCollectionToNetwork(networkID, dc.Name); err != nil {
        return err
    }

    dc.networkID = networkID
    dc.syncStates[networkID] = &SyncState{Collection: dc.Name, NetworkID: networkID, LocalVector: NewVectorClock(), LastSync: time.Now(), PendingOperations: nil, SyncInProgress: false}

    // Request initial sync
    return dc.requestSync()
}

func (dc *DistributedCollection) DetachFromNetwork() error {
    if dc.networkID == "" { return nil }
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
    entryType, _ := doc["entryType"].(EntryType) // Assuming validation already happened

    // For MEMORY entries, we separate the blob for local storage.
    if entryType == EntryTypeMemory {
        if payload, ok := doc["payload"].(map[string]interface{}); ok {
            if blobData, hasBlob := payload["blob"]; hasBlob {
                // 1. Save blobData to a local file in the app data directory.
                //    This is a placeholder for the actual file I/O logic.
                //    blobPath, err := saveBlobToLocalStorage(id, blobData)
                //    if err != nil { return nil, err }

                // 2. The payload is modified for synchronization. The blob data is
                //    removed, and a reference is stored instead. The metadata and
                //    vector remain in the payload to be synchronized.
                // payload["blobRef"] = blobPath
                delete(payload, "blob")
                doc["payload"] = payload // Update the document with the modified payload
            }
        }
    }

    // The local collection handles persistence. The underlying adapter is responsible
    // for the actual file I/O for blobs.
    inserted, err := dc.local.Insert(doc)
    if err != nil {
        return nil, err
    }

    if dc.networkID != "" {
        opPayload := ToDistributed(inserted, dc.network.GetPeerID())
        opPayload.EntryType = entryType // Ensure EntryType is set on the distributed doc

        op := CRDTOperation{
            ID:         fmt.Sprintf("%s-%d-%d", dc.network.GetPeerID(), time.Now().UnixMilli(), time.Now().UnixNano()%1000),
            Type:       OpInsert,
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
    if err != nil { return 0, err }
    if dc.networkID != "" && affected > 0 {
        d, _ := dc.local.Find(id)
        op := CRDTOperation{ID: fmt.Sprintf("%s-%d", dc.network.GetPeerID(), time.Now().UnixMilli()), Type: OpUpdate, Collection: dc.Name, DocumentID: id, Data: ToDistributed(d, dc.network.GetPeerID()), Vector: dc.getCurrentVector(), Timestamp: time.Now().UnixMilli(), PeerID: dc.network.GetPeerID()}
        dc.broadcastOperation(op)
    }
    return affected, nil
}

func (dc *DistributedCollection) Delete(id string) (int, error) {
    d, _ := dc.local.Find(id)
    affected, err := dc.local.Delete(id)
    if err != nil { return 0, err }
    if dc.networkID != "" && affected > 0 {
        op := CRDTOperation{ID: fmt.Sprintf("%s-%d", dc.network.GetPeerID(), time.Now().UnixMilli()), Type: OpDelete, Collection: dc.Name, DocumentID: id, Data: &DistributedDocument{ID: id, Deleted: true}, Vector: dc.getCurrentVector(), Timestamp: time.Now().UnixMilli(), PeerID: dc.network.GetPeerID()}
        dc.broadcastOperation(op)
    }
    return affected, nil
}

func (dc *DistributedCollection) Find(id string) (map[string]interface{}, error) { return dc.local.Find(id) }
func (dc *DistributedCollection) FindAll() ([]map[string]interface{}, error) {
    dc.local.mu.RLock()
    defer dc.local.mu.RUnlock()
    out := make([]map[string]interface{}, 0, len(dc.local.store))
    for _, d := range dc.local.store { out = append(out, cloneMap(d)) }
    return out, nil
}

func (dc *DistributedCollection) GetSyncState() *SyncState {
    if dc.networkID == "" { return nil }
    return dc.syncStates[dc.networkID]
}

func (dc *DistributedCollection) ForceSync() error { return dc.requestSync() }

// Private helpers

func (dc *DistributedCollection) broadcastOperation(op CRDTOperation) {
    if dc.networkID == "" { return }

    dc.mu.Lock()
    dc.operationLog = append(dc.operationLog, op)
    dc.pruneOperationLog()
    // increment local vector
    syncState := dc.syncStates[dc.networkID]
    syncState.LocalVector = Increment(syncState.LocalVector, dc.network.GetPeerID())
    dc.mu.Unlock()

    _ = dc.network.BroadcastMessage(dc.networkID, ProtocolMessage{Type: MsgOperation, NetworkID: dc.networkID, SenderID: dc.network.GetPeerID(), Timestamp: time.Now().UnixMilli(), Payload: map[string]interface{}{"collection": dc.Name, "operation": op}})
}

func (dc *DistributedCollection) handleRemoteOperation(op CRDTOperation) {
    // Apply CRDT operation to local document
    existing, _ := dc.local.Find(op.DocumentID)
    var existingDist *DistributedDocument
    if existing != nil { existingDist = ToDistributed(existing, op.PeerID) }

    result := ApplyOperation(existingDist, op)

    if result == nil {
        // delete
        _ = dc.local.Delete(op.DocumentID)
    } else if result.Deleted {
        _ = dc.local.Delete(op.DocumentID)
    } else {
        // upsert
        _ , _ = dc.local.Insert(ToRegular(result))
    }

    // merge vector
    if dc.networkID != "" {
        syncState := dc.syncStates[dc.networkID]
        syncState.LocalVector = Merge(syncState.LocalVector, op.Vector)
    }
}

func (dc *DistributedCollection) requestSync() error {
    if dc.networkID == "" { return errors.New("not attached to network") }
    syncState := dc.syncStates[dc.networkID]
    if syncState.SyncInProgress { return nil }
    syncState.SyncInProgress = true

    _ = dc.network.BroadcastMessage(dc.networkID, ProtocolMessage{Type: MsgSyncRequest, NetworkID: dc.networkID, SenderID: dc.network.GetPeerID(), Timestamp: time.Now().UnixMilli(), Payload: map[string]interface{}{"collection": dc.Name, "vector": syncState.LocalVector}})

    // Clear flag after timeout
    go func() {
        time.Sleep(10 * time.Second)
        syncState.SyncInProgress = false
    }()

    return nil
}

func (dc *DistributedCollection) handleSyncRequest(msg ProtocolMessage) {
    payload, _ := msg.Payload.(map[string]interface{})
    remoteVector, _ := payload["vector"].(map[string]interface{})

    // convert remoteVector into VectorClock
    rv := make(VectorClock)
    for k, v := range remoteVector {
        switch val := v.(type) {
        case float64:
            rv[k] = int64(val)
        case int64:
            rv[k] = val
        }
    }

    // find missing ops
    missing := make([]CRDTOperation, 0)
    for _, op := range dc.operationLog {
        remoteClock := int64(0)
        if vv, ok := rv[op.PeerID]; ok { remoteClock = vv }
        opClock := int64(0)
        if vv, ok := op.Vector[op.PeerID]; ok { opClock = vv }
        if opClock > remoteClock { missing = append(missing, op) }
    }

    _ = dc.network.SendToPeer(msg.SenderID, dc.networkID, ProtocolMessage{Type: MsgSyncResponse, NetworkID: dc.networkID, SenderID: dc.network.GetPeerID(), Timestamp: time.Now().UnixMilli(), Payload: map[string]interface{}{"collection": dc.Name, "operations": missing, "vector": dc.syncStates[dc.networkID].LocalVector}})
}

func (dc *DistributedCollection) handleSyncResponse(msg ProtocolMessage) {
    payload, _ := msg.Payload.(map[string]interface{})
    opsIface, _ := payload["operations"].([]interface{})

    for _, oi := range opsIface {
        b, _ := jsonMarshal(oi)
        var op CRDTOperation
        _ = jsonUnmarshal(b, &op)
        dc.handleRemoteOperation(op)
    }

    if dc.networkID != "" {
        syncState := dc.syncStates[dc.networkID]
        syncState.SyncInProgress = false
        syncState.LastSync = time.Now()
    }
}

func (dc *DistributedCollection) getCurrentVector() VectorClock {
    if dc.networkID == "" { return NewVectorClock() }
    if s, ok := dc.syncStates[dc.networkID]; ok { return Clone(s.LocalVector) }
    return NewVectorClock()
}

func (dc *DistributedCollection) pruneOperationLog() {
    if len(dc.operationLog) > dc.maxLogSize {
        dc.operationLog = dc.operationLog[len(dc.operationLog)-dc.maxLogSize:]
    }
}

// Helper JSON marshalling/unmarshalling helpers used by the example to avoid importing encoding/json repeatedly
func jsonMarshal(v interface{}) ([]byte, error) { return json.Marshal(v) }
func jsonUnmarshal(b []byte, v interface{}) error { return json.Unmarshal(b, v) }
```

---

### 6) Distributed Database (distributed_database.go)

```go
package distributed

import (
    "context"
    "errors"
    "sync"
)

type DistributedDbOptions struct {
    Distributed struct {
        Enabled bool
        NetworkID string
        BootstrapPeers []string
    }
}

type DistributedDatabase struct {
    network    Network
    distributed bool
    collections map[string]*DistributedCollection
    mu sync.Mutex
}

func NewDistributedDatabase(ctx context.Context, opts DistributedDbOptions) (*DistributedDatabase, error) {
    nm := NewNetworkManager(ctx)
    db := &DistributedDatabase{network: nm, distributed: opts.Distributed.Enabled, collections: make(map[string]*DistributedCollection)}
    if db.distributed {
        if err := nm.Initialize(); err != nil { return nil, err }
        if opts.Distributed.NetworkID != "" {
            if len(opts.Distributed.BootstrapPeers) > 0 {
                if err := nm.JoinNetwork(opts.Distributed.NetworkID, opts.Distributed.BootstrapPeers); err != nil { return nil, err }
            } else {
                _, err := nm.CreateNetwork(NetworkConfig{NetworkID: opts.Distributed.NetworkID, Name: "Network " + opts.Distributed.NetworkID})
                if err != nil { return nil, err }
            }
        }
    }
    return db, nil
}

func (db *DistributedDatabase) Collection(name string) *DistributedCollection {
    db.mu.Lock()
    defer db.mu.Unlock()

    if c, ok := db.collections[name]; ok { return c }
    c := NewDistributedCollection(name, db.network)
    db.collections[name] = c
    return c
}

func (db *DistributedDatabase) CreateNetwork(cfg NetworkConfig) (string, error) {
    if db.network == nil { return "", errors.New("network manager not initialized") }
    return db.network.CreateNetwork(cfg)
}

func (db *DistributedDatabase) JoinNetwork(networkID string, bootstrapPeers []string) error {
    if db.network == nil { return errors.New("network manager not initialized") }
    return db.network.JoinNetwork(networkID, bootstrapPeers)
}

func (db *DistributedDatabase) LeaveNetwork(networkID string) error {
    if db.network == nil { return errors.New("network manager not initialized") }
    return db.network.LeaveNetwork(networkID)
}

func (db *DistributedDatabase) AddCollectionToNetwork(networkID string, collectionName string) error {
    c := db.Collection(collectionName)
    if c == nil { return errors.New("collection not found") }
    return c.AttachToNetwork(networkID)
}

func (db *DistributedDatabase) RemoveCollectionFromNetwork(collectionName string) error {
    c := db.collections[collectionName]
    if c == nil { return nil }
    return c.DetachFromNetwork()
}

func (db *DistributedDatabase) GetNetworkManager() Network { return db.network }

func (db *DistributedDatabase) Shutdown() error {
    if db.network == nil { return nil }
    return db.network.Shutdown()
}
```
---

## Testing Implementation

Tests use Go's `testing` package and table-driven tests. Example tests below are simplified and focus on vector clock and CRDT behavior.

### Vector Clock Tests (vector_clock_test.go)

```go
package distributed_test

import (
    "testing"
    "github.com/your/module/distributed"
)

func TestVectorClockBasics(t *testing.T) {
    c := distributed.NewVectorClock()
    if len(c) != 0 { t.Fatalf("expected empty clock") }

    c = distributed.Increment(c, "peer1")
    if c["peer1"] != 1 { t.Fatalf("expected 1") }

    c = distributed.Increment(c, "peer1")
    if c["peer1"] != 2 { t.Fatalf("expected 2") }
}
```

### CRDT Resolver Tests (crdt_resolver_test.go)

```go
package distributed_test

import (
    "testing"
    "time"
    "github.com/your/module/distributed"
)

func TestResolveConflict(t *testing.T) {
    now := time.Now().UnixMilli()
    local := &distributed.DistributedDocument{ID: "1", Payload: map[string]interface{}{"name":"Local"}, Vector: distributed.VectorClock{"peer1":2}, Timestamp: now}
    remote := &distributed.DistributedDocument{ID: "1", Payload: map[string]interface{}{"name":"Remote"}, Vector: distributed.VectorClock{"peer1":1}, Timestamp: now-1000}

    res := distributed.ResolveConflict(local, remote)
    if res.Payload["name"] != "Local" { t.Fatalf("expected Local winner") }
}
```

> Tests for network manager and integration tests are included as skeletons here—real network tests should be run in a controlled environment or use mocks for libp2p components.

### Distributed Collection Tests (distributed_collection_test.go)

```go
package distributed_test

import (
    "context"
    "testing"
    "sync"
    "time"

    "github.com/your/module/distributed"
)

// MockNetwork is a lightweight in-memory network for unit tests that routes messages
type MockNetwork struct {
    peerID   string
    mu       sync.RWMutex
    handlers map[distributed.MessageType][]distributed.MessageHandler
}

// registry of networks by networkID to enable BroadcastMessage between mocks
var mockRegistry = struct {
    mu sync.RWMutex
    m  map[string]map[string]*MockNetwork
}{m: make(map[string]map[string]*MockNetwork)}

func NewMockNetwork(peerID string) *MockNetwork {
    return &MockNetwork{peerID: peerID, handlers: make(map[distributed.MessageType][]distributed.MessageHandler)}
}

func (m *MockNetwork) Initialize() error { return nil }
func (m *MockNetwork) CreateNetwork(cfg distributed.NetworkConfig) (string, error) {
    return cfg.NetworkID, nil
}
func (m *MockNetwork) JoinNetwork(networkID string, bootstrapPeers []string) error { return nil }
func (m *MockNetwork) LeaveNetwork(networkID string) error { return nil }
func (m *MockNetwork) AddCollectionToNetwork(networkID, collectionName string) error {
    mockRegistry.mu.Lock()
    defer mockRegistry.mu.Unlock()
    if _, ok := mockRegistry.m[networkID]; !ok { mockRegistry.m[networkID] = make(map[string]*MockNetwork) }
    mockRegistry.m[networkID][m.peerID] = m
    return nil
}
func (m *MockNetwork) RemoveCollectionFromNetwork(networkID, collectionName string) error {
    mockRegistry.mu.Lock()
    defer mockRegistry.mu.Unlock()
    if peers, ok := mockRegistry.m[networkID]; ok { delete(peers, m.peerID) }
    return nil
}
func (m *MockNetwork) GetNetworkCollections(networkID string) []string { return nil }
func (m *MockNetwork) BroadcastMessage(networkID string, msg distributed.ProtocolMessage) error {
    mockRegistry.mu.RLock()
    defer mockRegistry.mu.RUnlock()
    if peers, ok := mockRegistry.m[networkID]; ok {
        for _, p := range peers {
            // deliver to each peer's handlers for the message type
            p.mu.RLock()
            handlers := p.handlers[msg.Type]
            p.mu.RUnlock()
            for _, h := range handlers { go h(msg) }
        }
    }
    return nil
}
func (m *MockNetwork) SendToPeer(peerID string, networkID string, msg distributed.ProtocolMessage) error {
    mockRegistry.mu.RLock()
    defer mockRegistry.mu.RUnlock()
    if peers, ok := mockRegistry.m[networkID]; ok {
        if p, ok2 := peers[peerID]; ok2 {
            p.mu.RLock()
            handlers := p.handlers[msg.Type]
            p.mu.RUnlock()
            for _, h := range handlers { go h(msg) }
            return nil
        }
    }
    return nil
}
func (m *MockNetwork) OnMessage(mt distributed.MessageType, handler distributed.MessageHandler) { m.mu.Lock(); m.handlers[mt] = append(m.handlers[mt], handler); m.mu.Unlock() }
func (m *MockNetwork) GetNetworkStats(networkID string) *distributed.NetworkStats { return nil }
func (m *MockNetwork) GetNetworks() []*distributed.NetworkConfig { return nil }
func (m *MockNetwork) GetPeerID() string { return m.peerID }
func (m *MockNetwork) Shutdown() error { return nil }

func TestDistributedCollectionLocalOps(t *testing.T) {
    net := NewMockNetwork("peer1")
    dc := distributed.NewDistributedCollection("users", net)

    ctx := context.Background()

    doc := map[string]interface{}{"id": "u1", "name": "Alice", "age": 30}
    inserted, err := dc.Insert(ctx, doc)
    if err != nil { t.Fatalf("insert failed: %v", err) }
    if inserted["id"] != "u1" { t.Fatalf("unexpected id") }

    found, _ := dc.Find("u1")
    if found == nil || found["name"] != "Alice" { t.Fatalf("expected Alice") }

    // Update
    updated, err := dc.Update("u1", map[string]interface{}{"age": 31})
    if err != nil { t.Fatalf("update failed: %v", err) }
    if updated != 1 { t.Fatalf("expected 1 updated, got %d", updated) }

    f2, _ := dc.Find("u1")
    if f2["age"] != 31 { t.Fatalf("expected age 31") }

    // Delete
    del, err := dc.Delete("u1")
    if err != nil { t.Fatalf("delete failed: %v", err) }
    if del != 1 { t.Fatalf("expected 1 deleted") }

    f3, _ := dc.Find("u1")
    if f3 != nil { t.Fatalf("expected document removed") }
}

func TestDistributedCollectionSyncBetweenPeers(t *testing.T) {
    net1 := NewMockNetwork("peer1")
    net2 := NewMockNetwork("peer2")

    dc1 := distributed.NewDistributedCollection("users", net1)
    dc2 := distributed.NewDistributedCollection("users", net2)

    // Both peers join the same mock network
    _ = net1.AddCollectionToNetwork("net-A", "users")
    _ = net2.AddCollectionToNetwork("net-A", "users")

    // Attach to network
    _ = dc1.AttachToNetwork("net-A")
    _ = dc2.AttachToNetwork("net-A")

    // Insert on peer1 and ensure peer2 receives and applies it
    _, err := dc1.Insert(context.Background(), map[string]interface{}{"id": "u10", "name": "Bob"})
    if err != nil { t.Fatalf("insert failed: %v", err) }

    // wait to allow goroutines to process
    time.Sleep(50 * time.Millisecond)

    f, _ := dc2.Find("u10")
    if f == nil || f["name"] != "Bob" { t.Fatalf("expected Bob on peer2") }
}
```


---

## Dependencies (go.mod snippet)

```go
module github.com/your/module

go 1.20

require (
    github.com/libp2p/go-libp2p v0.34.0
    github.com/libp2p/go-libp2p-kad-dht v0.15.0
    github.com/multiformats/go-multiaddr v0.3.0
)
```

---

## Implementation Checklist ✅

- [ ] Implement vector clock manager 
- [ ] Implement CRDT resolver with conflict resolution 
- [ ] Implement network manager with go-libp2p 
- [ ] Implement distributed collection 
- [ ] Implement distributed database 
- [ ] Write unit tests for vector clock, CRDT, and distributed collection (included)
- [ ] Write integration tests for multi-peer scenarios (requires environment / full libp2p)
- [ ] Update documentation with distributed usage examples (this file)
- [ ] Add network monitoring and debugging tools
- [ ] Implement encryption for sensitive data (note: uses libp2p noise; additional access control required)
- [ ] Add performance benchmarks for distributed operations


---

## Usage Example (Go)

```go
package main

import (
    "context"
    "fmt"
    "github.com/your/module/distributed"
)

func main() {
    ctx := context.Background()
    db := distributed.NewDistributedDatabase(ctx, distributed.DistributedDbOptions{})

    // Create network
    db.CreateNetwork(distributed.NetworkConfig{NetworkID: "consortium-1", Name: "Consortium 1"})

    // Collections
    users := db.Collection("users")

    // Insert
    users.Insert(ctx, map[string]interface{}{"id":"u1", "name":"Alice"})

    fmt.Println("Inserted user Alice. Next: attach collections and run sync in your app runtime.")
}
```

---

## Security Considerations

- **Network Encryption:** Use go-libp2p's noise/TLS for encrypted transport
- **Shared Secrets:** Optionally enforce shared secrets for join authorization
- **Collection Access Control:** Only peers with access to a network should sync its collections
- **Data Validation:** Validate incoming ops before applying
- **DoS Protection:** Rate-limit message processing and implement quotas

## Performance Optimization

- Operation log pruning
- Partial replication strategies
- Lazy synchronization and batching
- Compress large payloads before transmission
- Add benchmarks to identify hot-paths

## Future Enhancements

- Leader election (Raft) for leader-based replication
- Sharding collections across nodes for scalability
- WebRTC transport for browser-to-browser connections
- Merkle trees for efficient diff checks
- Tombstone garbage collection and compaction

---

If you'd like, I can also:
- run `gofmt` and `go vet` on the generated files,
- scaffold actual package files under `packages/core/distributed/` in Go, or
- add unit tests and CI config to run them.
