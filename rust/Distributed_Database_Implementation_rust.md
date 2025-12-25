# NebulusDB Distributed Database Implementation (Rust)

## Overview

This document outlines a Rust translation of the NebulusDB distributed database design. It maps the TypeScript reference implementation to idiomatic Rust, focusing on detailed types, CRDT conflict resolution, vector clocks, the network abstraction (libp2p-based), distributed collections and database wrappers, and test examples.

> The TypeScript implementation is the source of truth; this document provides a Rust counterpart with recommended crates, function signatures, and unit test examples to support a Rust implementation.

## Architecture Goals

- **Local-First:** All operations work locally with background synchronization ✅
- **Network Isolation:** Each peer network is an independent consortium with separate collections ✅
- **Dynamic Membership:** Collections can be added/removed from networks at runtime ✅
- **Conflict Resolution:** CRDT-based automatic conflict resolution with vector clocks ✅
- **Type Safety:** Full Rust type safety and ownership guarantees ✅
- **Backward Compatible:** Existing non-distributed functionality should be kept separable from distributed logic ✅

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

    The synchronized document (`DistributedDocument`) contains metadata about the blob (like its path or a content hash), but the blob data itself is **not** synchronized across the network in this version..


### Design Rationale: Why Blobs Are Not Synchronized

In this architecture, only the metadata and vector representation for `MEMORY` entries are synchronized across peers. The raw data blob is stored only on the local system where it was created. This design was chosen for several key reasons related to performance and efficiency in a distributed system:

1.  **Network Efficiency:** Broadcasting large binary files (blobs) to every peer upon every change would consume significant bandwidth, leading to slow synchronization and a less responsive network.
2.  **Storage Overhead:** Requiring every peer to store a copy of every blob from all other peers would lead to excessive storage consumption. This design allows peers to only store the blobs they explicitly need.
3.  **On-Demand Fetching:** The synchronized metadata includes a reference to the blob (like a path or content hash). A peer that needs the full blob can use this reference to request it directly from the origin peer or a future dedicated storage layer (like IPFS). This pattern of "synchronizing discovery, not data" is a standard practice for keeping distributed systems fast and scalable.

---


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



## Technology Stack (Rust)

- **P2P Layer:** rust-libp2p (libp2p in Rust)
- **Async runtime:** tokio
- **Serialization:** serde / serde_json
- **CRDTs / Conflict Resolution:** custom CRDT resolver based on vector clocks
- **Transport:** TCP / WebRTC (planned) / QUIC (optional)
- **Discovery:** mDNS, DHT (kad)
- **Encryption:** libp2p's noise / TLS where applicable

---

## Core Components (Rust)

### 1. Network Types and Data Structures

**Module:** `core::distributed::types`

```rust
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Vector clock: mapping of peer id -> counter
pub type VectorClock = HashMap<String, u64>;

/// Type of entry
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum EntryType {
    MEMORY,
    AUTH,
}

/// Document stored in a distributed collection
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DistributedDocument {
    // application fields (id, etc.) are dynamic — represented as JSON map in simple implementations
    #[serde(flatten)]
    pub payload: serde_json::Value,

    /// type of entry
    pub _entry_type: EntryType,

    /// vector clock
    pub _vector: VectorClock,

    /// unix ms
    pub _timestamp: u128,

    /// origin peer id
    pub _peer_id: String,

    /// Optional stage marker (e.g. Some("post-pending")). When set to "post-pending" the document is
    /// staged locally and will be converted into a KNIRVGRAPH transaction during the next sync.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub _stage: Option<String>,

    /// tombstone flag
    #[serde(default)]
    pub _deleted: bool,
}

/// CRDT operation types
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum OperationType {
    Insert,
    Update,
    Delete,
}

/// A CRDT operation for synchronization
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CrdtOperation {
    pub id: String,
    pub op_type: OperationType,
    pub collection: String,
    pub document_id: String,
    pub data: serde_json::Value,
    pub vector: VectorClock,
    pub timestamp: u128,
    pub peer_id: String,
}

/// Network configuration
/// Additional fields control posting/staging behavior:
///  - default_posting_network: the network to which staged entries are posted (e.g. "knirvgraph").
///  - auto_post_classifications: classifications auto-staged for posting (e.g. Error, Context, Idea).
///  - private_by_default: when true, entries are private unless staged or explicitly configured.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NetworkConfig {
    pub network_id: String,
    pub name: String,
    pub collections: Vec<String>,
    pub bootstrap_peers: Vec<String>, // multiaddrs as strings for now

    /// Default posting target for staged entries (e.g. Some("knirvgraph"))
    pub default_posting_network: Option<String>,

    /// Entry classifications that will be auto-staged for posting (e.g., Error, Context, Idea)
    pub auto_post_classifications: Vec<EntryType>,

    /// When true, entries are private by default unless staged or configured otherwise
    pub private_by_default: bool,

    pub encryption_enabled: bool,
    pub replication_factor: usize,
    pub replication_strategy: String, // "full" | "partial" | "leader"
    pub discovery_mdns: bool,
    pub discovery_bootstrap: bool,
}

/// Peer info
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PeerInfo {
    pub peer_id: String,
    pub addresses: Vec<String>,
    pub protocols: Vec<String>,
    pub latency_ms: Option<u64>,
    pub last_seen: u128,
    pub collections: Vec<String>,
}

/// Sync state for a collection
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SyncState {
    pub collection: String,
    pub network_id: String,
    pub local_vector: VectorClock,
    pub last_sync: u128,
    pub pending_operations: Vec<CrdtOperation>,
    /// IDs of documents staged with `_stage == Some("post-pending")` for posting during next sync
    pub staged_entries: Vec<String>,
    pub sync_in_progress: bool,
}

/// Network statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NetworkStats {
    pub network_id: String,
    pub connected_peers: usize,
    pub total_peers: usize,
    pub collections_shared: usize,
    pub operations_sent: u64,
    pub operations_received: u64,
    pub bytes_transferred: u64,
    pub average_latency_ms: u64,
}

/// Message types for peer communication
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum MessageType {
    SyncRequest,
    SyncResponse,
    Operation,
    Heartbeat,
    CollectionAnnounce,
    CollectionRequest,
}

/// Protocol message wrapper
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProtocolMessage {
    pub msg_type: MessageType,
    pub network_id: String,
    pub sender_id: String,
    pub timestamp: u128,
    pub payload: serde_json::Value,
}
```

> Implementation notes: Using `serde_json::Value` for document payloads makes the types flexible; for a stricter schema, replace with concrete typed structs.

---

### Post-Pending Staging

To support workflows where local entries should be posted to the KNIRVGRAPH as transactions during the next sync, a document may include an optional `_stage` field set to `Some("post-pending")`.

**Important:** **Only** documents that are staged (`_stage == Some("post-pending")`) are synchronized with a chosen network. By default, entries classified as **Error**, **Context**, and **Idea** will be auto-staged for posting to the `KNIRVGRAPH` (unless the `NetworkConfig` overrides this behavior). All other entries are **private by default** and will not be synchronized unless explicitly staged or configured otherwise.

Behavior summary:

- Documents with `_stage == Some("post-pending")` remain local and are **not** published as regular CRDT operations to peers; they are handled by the staged-post flow instead.
- The `SyncState::staged_entries` vector holds doc IDs of staged documents for each collection/network.
- Use `NetworkConfig` fields to control defaults (e.g. `default_posting_network`, `auto_post_classifications`, `private_by_default`).
- On the next successful sync for a collection the sync routine will:
  1. Collect staged documents by ID from `staged_entries`.
  2. Convert each `DistributedDocument` to a transaction for the configured posting network and submit it using the graph client.
  3. On success, clear `_stage` and remove the ID from `staged_entries` (persisting the change).

Example (Rust pseudocode):

```rust
// Auto-stage document on save if classification matches network config
fn save_document(db: &mut Db, collection: &str, mut doc: DistributedDocument) -> Result<()> {
    if db.network_config.auto_post_classifications.contains(&doc.payload["classification"].as_str().unwrap_or_default().into()) {
        doc._stage = Some("post-pending".to_string());
        let mut s = db.load_sync_state(collection)?;
        s.staged_entries.push(doc.id.clone());
        db.save_sync_state(collection, &s)?;
    }

    db.save_document(collection, &doc)
}

// Sync handler
async fn handle_sync(sync_state: &mut SyncState, db: &mut Db, graph: &GraphClient) {
    for doc_id in sync_state.staged_entries.clone() {
        if let Ok(doc) = db.get_document(&sync_state.collection, &doc_id) {
            if doc._stage.as_deref() == Some("post-pending") {
                let tx = build_knirv_tx(&doc);
                if graph.submit_transaction(tx).await.is_ok() {
                    let mut doc = doc;
                    doc._stage = None;
                    db.save_document(&sync_state.collection, &doc).ok();
                    sync_state.staged_entries.retain(|id| id != &doc_id);
                }
            }
        }
    }
    db.save_sync_state(&sync_state.collection, &sync_state).ok();
}
```

Note: Posting to the KNIRVGRAPH is orthogonal to CRDT synchronization—CRDT continues to reconcile document state across peers, while `post-pending` expresses an explicit local intent to publish a document as an on-chain/graph transaction.

### 2. Vector Clock Implementation

**Module:** `core::distributed::vector_clock`

```rust
use crate::distributed::types::VectorClock;
use std::cmp::max;

#[derive(Debug, PartialEq, Eq)]
pub enum CompareResult {
    Before,
    After,
    Concurrent,
    Equal,
}

pub struct VectorClockManager;

impl VectorClockManager {
    pub fn create() -> VectorClock {
        VectorClock::new()
    }

    pub fn increment(clock: &mut VectorClock, peer_id: &str) {
        let entry = clock.entry(peer_id.to_string()).or_insert(0);
        *entry += 1;
    }

    pub fn merge(clock1: &VectorClock, clock2: &VectorClock) -> VectorClock {
        let mut merged = VectorClock::new();
        for key in clock1.keys().chain(clock2.keys()) {
            let v1 = *clock1.get(key).unwrap_or(&0);
            let v2 = *clock2.get(key).unwrap_or(&0);
            merged.insert(key.clone(), max(v1, v2));
        }
        merged
    }

    pub fn compare(clock1: &VectorClock, clock2: &VectorClock) -> CompareResult {
        let mut has_greater = false;
        let mut has_less = false;

        for key in clock1.keys().chain(clock2.keys()) {
            let v1 = *clock1.get(key).unwrap_or(&0);
            let v2 = *clock2.get(key).unwrap_or(&0);

            if v1 > v2 {
                has_greater = true;
            }
            if v1 < v2 {
                has_less = true;
            }
        }

        match (has_greater, has_less) {
            (false, false) => CompareResult::Equal,
            (true, false) => CompareResult::After,
            (false, true) => CompareResult::Before,
            _ => CompareResult::Concurrent,
        }
    }

    pub fn happens_before(clock1: &VectorClock, clock2: &VectorClock) -> bool {
        matches!(Self::compare(clock1, clock2), CompareResult::Before | CompareResult::Equal)
    }
}
```

---

### 3. CRDT Conflict Resolution

**Module:** `core::distributed::crdt_resolver`

```rust
use crate::distributed::types::{DistributedDocument, CrdtOperation};
use crate::distributed::vector_clock::{VectorClockManager, CompareResult};
use serde_json::Value;
use std::collections::HashMap;

pub struct CrdtResolver;

impl CrdtResolver {
    /// Resolve two distributed document versions using vector clocks + timestamp tie-breakers.
    /// Note: Documents with `_stage == Some("post-pending")` are a local intent and are not
    /// emitted as CRDT operations until they are posted to KNIRVGRAPH during the sync flow.
    pub fn resolve_conflict(local: &DistributedDocument, remote: &DistributedDocument) -> DistributedDocument {
        // Handle deletions first
        if remote._deleted && !local._deleted {
            let cmp = VectorClockManager::compare(&local._vector, &remote._vector);
            if matches!(cmp, CompareResult::Before | CompareResult::Concurrent) {
                return remote.clone();
            }
            return local.clone();
        }

        if local._deleted && !remote._deleted {
            let cmp = VectorClockManager::compare(&remote._vector, &local._vector);
            if matches!(cmp, CompareResult::Before | CompareResult::Concurrent) {
                return local.clone();
            }
            return remote.clone();
        }

        let cmp = VectorClockManager::compare(&local._vector, &remote._vector);
        match cmp {
            CompareResult::After => local.clone(),
            CompareResult::Before => remote.clone(),
            CompareResult::Equal => local.clone(),
            CompareResult::Concurrent => {
                // Tie-breaker by timestamp then peer id
                if local._timestamp > remote._timestamp {
                    Self::merge_documents(local, remote)
                } else if local._timestamp < remote._timestamp {
                    Self::merge_documents(remote, local)
                } else {
                    if local._peer_id > remote._peer_id {
                        Self::merge_documents(local, remote)
                    } else {
                        Self::merge_documents(remote, local)
                    }
                }
            }
        }
    }

    /// Winner-based merge: winner fields win, but preserve non-conflicting fields from loser
    fn merge_documents(winner: &DistributedDocument, loser: &DistributedDocument) -> DistributedDocument {
        let mut merged = winner.clone();
        merged._vector = VectorClockManager::merge(&winner._vector, &loser._vector);

        // Merge payloads: for fields missing in winner, take from loser
        if let (Value::Object(mut wmap), Value::Object(lmap)) = (winner.payload.clone(), loser.payload.clone()) {
            for (k, v) in lmap.into_iter() {
                wmap.entry(k).or_insert(v);
            }
            merged.payload = Value::Object(wmap);
        }

        merged
    }

    /// Apply an operation to a document (insert/update/delete)
    pub fn apply_operation(doc: Option<DistributedDocument>, op: &CrdtOperation) -> Option<DistributedDocument> {
        match op.op_type {
            crate::distributed::types::OperationType::Insert |
            crate::distributed::types::OperationType::Update => {
                let new_doc = if let Some(mut d) = doc {
                    // Compare vectors
                    let cmp = VectorClockManager::compare(&d._vector, &op.vector);
                    if matches!(cmp, CompareResult::Before | CompareResult::Concurrent) {
                        // Merge fields from op.data into d.payload
                        if let (Value::Object(mut dmap), Value::Object(omap)) = (d.payload.clone(), op.data.clone()) {
                            for (k, v) in omap.into_iter() {
                                dmap.insert(k, v);
                            }
                            d.payload = Value::Object(dmap);
                        }
                        d._vector = VectorClockManager::merge(&d._vector, &op.vector);
                        d._timestamp = std::cmp::max(d._timestamp, op.timestamp);
                        Some(d)
                    } else {
                        Some(d)
                    }
                } else {
                    // Insert new document from op.data
                    let mut payload = op.data.clone();
                    // Ensure metadata fields exist
                    let _vector = op.vector.clone();
                    let _timestamp = op.timestamp;
                    let _peer_id = op.peer_id.clone();

                    // Build DistributedDocument
                    Some(DistributedDocument { payload, _vector, _timestamp, _peer_id, _deleted: false })
                };

                new_doc
            }

            crate::distributed::types::OperationType::Delete => {
                if doc.is_none() {
                    return None;
                }
                let mut d = doc.unwrap();
                let cmp = VectorClockManager::compare(&d._vector, &op.vector);
                if matches!(cmp, CompareResult::Before | CompareResult::Concurrent) {
                    d._deleted = true;
                    d._vector = VectorClockManager::merge(&d._vector, &op.vector);
                    d._timestamp = std::cmp::max(d._timestamp, op.timestamp);
                }
                Some(d)
            }
        }
    }

    /// Convert a regular JSON document to distributed doc
    pub fn to_distributed_document(payload: serde_json::Value, peer_id: &str) -> DistributedDocument {
        let mut vector = VectorClockManager::create();
        vector.insert(peer_id.to_string(), 1);

        DistributedDocument {
            payload,
            _vector: vector,
            _timestamp: chrono::Utc::now().timestamp_millis() as u128,
            _peer_id: peer_id.to_string(),
            _deleted: false,
        }
    }

    /// Convert distributed document back to regular JSON value
    pub fn to_regular_document(doc: &DistributedDocument) -> serde_json::Value {
        // Remove internal keys (already kept in separate fields) — payload is returned as-is
        doc.payload.clone()
    }
}
```

> Notes: We use `serde_json::Value` for flexible payloads; for typed schemas, branch into typed merges.

---

### 4. Network Manager (Outline)

**Module:** `core::distributed::network_manager`

This is an outline and suggested API for a Rust `NetworkManager` using rust-libp2p and tokio. A full implementation requires integration with libp2p's swarm, behaviours, and async message passing.

Key features to mirror from TypeScript:
- Create and manage networks
- Start / stop the libp2p node
- Handle peer discovery, connect/disconnect, and protocol handling
- Broadcast messages to peers and send to specific peer
- Track network statistics and peer info

```rust
// Pseudocode / API sketch

use async_trait::async_trait;
use crate::distributed::types::*;
use std::sync::Arc;

#[async_trait]
pub trait NetworkManagerTrait: Send + Sync {
    async fn initialize(&mut self) -> anyhow::Result<()>;
    async fn create_network(&mut self, config: NetworkConfig) -> anyhow::Result<String>;
    async fn join_network(&mut self, network_id: &str, bootstrap_peers: &[String]) -> anyhow::Result<()>;
    async fn leave_network(&mut self, network_id: &str) -> anyhow::Result<()>;
    async fn broadcast_message(&self, network_id: &str, message: &ProtocolMessage) -> anyhow::Result<()>;
    async fn send_to_peer(&self, peer_id: &str, network_id: &str, message: &ProtocolMessage) -> anyhow::Result<()>;
    fn on_message(&mut self, msg_type: MessageType, handler: Box<dyn Fn(ProtocolMessage) + Send + Sync>);
    fn get_peer_id(&self) -> String;
    async fn shutdown(&mut self) -> anyhow::Result<()>;
}

// Implementation notes:
// - Use libp2p::Swarm with a custom behaviour combining Gossipsub/mDNS/Kademlia as appropriate.
// - Use tokio channels to surface incoming messages to registered handlers.
// - Track stats in an Arc<Mutex<HashMap<String, NetworkStats>>>.
```

> Security: ensure encryption via libp2p's Noise and authenticated transports.

---

### 5. Distributed Collection (Outline)

**Module:** `core::distributed::distributed_collection`

This maps `DistributedCollection` behavior to Rust. Typical structure wraps a local collection (in-memory or persisted) and adds networking/synchronization features.

Key operations:
- attach_to_network
- detach_from_network
- insert / update / delete with broadcast of CRDT operations
- handle incoming operations and sync requests
- maintain an operation log and prune

```rust
// Sketch (non-executable outline)

pub struct DistributedCollection<LocalCol> {
    name: String,
    local: LocalCol,
    network_manager: Arc<dyn NetworkManagerTrait>,
    network_id: Option<String>,
    sync_states: HashMap<String, SyncState>,
    operation_log: Vec<CrdtOperation>,
    max_log_size: usize,
}

impl<LocalCol> DistributedCollection<LocalCol>
where
    LocalCol: CollectionTrait + Send + Sync + 'static,
{
    pub async fn attach_to_network(&mut self, network_id: &str) -> anyhow::Result<()> {
        if self.network_id.is_some() { anyhow::bail!("already attached"); }
        self.network_id = Some(network_id.to_string());
        self.network_manager.create_network(/* ... */).await?; // or join
        // Initialize sync state and request sync
        Ok(())
    }

    pub async fn insert(&mut self, mut doc: serde_json::Value) -> anyhow::Result<serde_json::Value> {
        let entry_type: EntryType = serde_json::from_value(
            doc.get("_entry_type").cloned().unwrap_or(serde_json::Value::Null)
        ).map_err(|_| anyhow::anyhow!("Document must contain a valid '_entry_type' ('MEMORY' or 'AUTH')"))?;

        if entry_type == EntryType::MEMORY {
            if let Some(payload) = doc.get_mut("payload").and_then(|p| p.as_object_mut()) {
                if let Some(blob_data) = payload.remove("blob") {
                    // 1. Save blob_data to a local file in the app data directory.
                    //    This is a placeholder for the actual file I/O logic.
                    //    let blob_path = save_blob_to_local_storage(id, blob_data).await?;

                    // 2. The payload is modified for synchronization. The blob data is
                    //    removed, and a reference is stored instead. The metadata and
                    //    vector remain in the payload to be synchronized.
                    // payload.insert("blobRef".to_string(), json!(blob_path));
                }
            }
        }

        // The local collection handles persistence. The underlying adapter is responsible
        // for the actual file I/O for blobs.
        let inserted = self.local.insert(doc).await?;

        if let Some(net) = &self.network_id {
            // build CrdtOperation and broadcast
            let op = CrdtOperation {
                // ... (id, collection, documentId, etc.)
                op_type: OperationType::Insert,
                data: inserted.clone(), // This would be the synchronized document
                // ...
            };
            // self.network_manager.broadcast_message(net, &op.into_protocol_message()).await?;
        }
        Ok(inserted)
    }

    // other methods mirror TypeScript implementation
}
```

> Note: while the outline shows the logic, an actual implementation should carefully handle concurrency, error handling, and back-pressure when broadcasting.

---

### 6. Distributed Database Wrapper

**Module:** `core::distributed::distributed_database`

Provide a database factory that optionally enables distribution. This mirrors the TypeScript `DistributedDatabase` class with helper functions to create networks and manage collections.

```rust
pub struct DistributedDatabase {
    network_manager: Arc<dyn NetworkManagerTrait>,
    distributed_enabled: bool,
    // map collection name -> collection instance
}

impl DistributedDatabase {
    pub fn new(options: DistributedDbOptions) -> Self { /* ... */ }

    pub fn collection(&mut self, name: &str) -> CollectionHandle { /* ... */ }

    pub async fn create_network(&self, config: NetworkConfig) -> anyhow::Result<String> { /* ... */ }

    pub async fn join_network(&self, network_id: &str, bootstrap_peers: &[String]) -> anyhow::Result<()> { /* ... */ }

    pub async fn shutdown(&self) -> anyhow::Result<()> { self.network_manager.shutdown().await }
}
```

---

## Testing Implementation (Rust examples)

### Vector Clock Tests

```rust
#[cfg(test)]
mod tests {
    use super::vector_clock::VectorClockManager;
    use super::vector_clock::CompareResult;

    #[test]
    fn create_and_increment() {
        let mut clock = VectorClockManager::create();
        assert_eq!(clock.len(), 0);

        VectorClockManager::increment(&mut clock, "peer1");
        assert_eq!(clock.get("peer1"), Some(&1u64));

        VectorClockManager::increment(&mut clock, "peer1");
        assert_eq!(clock.get("peer1"), Some(&2u64));
    }

    #[test]
    fn merge_and_compare() {
        let mut a = VectorClockManager::create();
        a.insert("peer1".to_string(), 3);
        a.insert("peer2".to_string(), 1);

        let mut b = VectorClockManager::create();
        b.insert("peer1".to_string(), 2);
        b.insert("peer2".to_string(), 4);
        b.insert("peer3".to_string(), 1);

        let merged = VectorClockManager::merge(&a, &b);
        assert_eq!(merged.get("peer1"), Some(&3u64));
        assert_eq!(merged.get("peer2"), Some(&4u64));
        assert_eq!(merged.get("peer3"), Some(&1u64));

        assert!(matches!(VectorClockManager::compare(&a, &b), CompareResult::Concurrent));
    }
}
```

### CRDT Resolver Tests (select examples)

```rust
#[cfg(test)]
mod crdt_tests {
    use super::crdt_resolver::CrdtResolver;
    use serde_json::json;

    #[test]
    fn to_and_from_distributed_doc() {
        let payload = json!({"id": "1", "name": "Test"});
        let peer = "peer1";
        let dd = CrdtResolver::to_distributed_document(payload.clone(), peer);
        assert_eq!(dd._peer_id, peer);
        assert_eq!(dd.payload, payload);
        assert!(dd._vector.contains_key(peer));
    }
}
```

> For integration tests that involve libp2p behaviour, use `tokio::test` and spin up lightweight nodes; tests can be heavier and are usually placed in an integration test suite.

---

## Cargo Dependencies (suggested)

Add to `Cargo.toml` for the distributed crate:

```toml
[dependencies]
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"
tokio = { version = "1.0", features = ["full"] }
libp2p = { version = "0.57", features = ["tcp-tokio", "dns-async-std"] }
async-trait = "0.1"
anyhow = "1.0"
futures = "0.3"
chrono = { version = "0.4", features = ["serde"] }
tracing = "0.1"

[dev-dependencies]
# tokio and test-related crates for async tests
```

> Note: Choose libp2p crate version that matches project constraints. Some features (WebRTC/QUIC) may require additional crates or nightly features.

---

## Implementation Checklist ✅

- [x] Translate types and interfaces to Rust structs and enums
- [ ] Implement vector clock manager with unit tests
- [ ] Implement CRDT resolver with conflict resolution and tests
- [ ] Implement NetworkManager abstraction (libp2p integration)
- [ ] Implement distributed collection layer
- [ ] Implement distributed database wrapper
- [ ] Write comprehensive unit and integration tests
- [ ] Update documentation and usage examples
- [ ] Add network monitoring and debugging tools
- [ ] Ensure encryption for sensitive data
- [ ] Add performance benchmarks for distributed operations

---

## Usage Example (Rust sketch)

```rust
// Sketch: creating a distributed DB in Rust
let db = DistributedDatabase::new(DistributedDbOptions { distributed: true, network_id: Some("my-app-network".to_string()), ..Default::default() });

let network_id = db.create_network(NetworkConfig { network_id: "consortium-1".into(), name: "Company Consortium".into(), bootstrap_peers: vec![], encryption_enabled: true, replication_factor: 3, replication_strategy: "full".into(), discovery_mdns: true, discovery_bootstrap: true, collections: vec![] }).await?;

let users = db.collection("users");
users.insert(serde_json::json!({"name": "Alice", "email": "alice@example.com"})).await?;

// Attach collection to network
users.attach_to_network(&network_id).await?;

// Trigger sync or rely on background synchronization
users.force_sync().await?;
```

---

## Security Considerations

- **Network Encryption:** Use libp2p's Noise transport or TLS tunnels
- **Shared Secrets:** Optionally require a joining token or pre-shared key (PSK) maintained outside the DHT discovery
- **Access Control:** Enforce authorization checks before applying operations
- **Validation:** Validate all incoming CRDT operations before applying
- **Rate Limits / DoS Protections:** Throttle expensive handlers and validate message sizes

---

## Performance Optimization

- Operation log pruning (keep most recent operations) with configurable max size
- Partial replication strategies to avoid full replication of large collections
- Lazy synchronization (on-demand or interval-based) to reduce chatter
- Compression of large payloads for network transfer
- Batch multiple operations into single messages

---

## Future Enhancements

- Leader election (Raft) for leader-based replication and stronger consistency
- Sharding of collections across nodes for scalability
- WebRTC support for browser-to-browser connections
- Merkle trees for efficient snapshot comparisons and synchronization
- Tombstone garbage collection for deleted documents

---

## Wrap Up

This document provides a thorough Rust translation sketch of the NebulusDB distributed database architecture and reference TypeScript implementation. It focuses on types, conflict resolution, vector clocks, and outlines how to integrate with rust-libp2p and tokio. The next steps are to implement the concrete modules, wire them into an existing database abstraction, and add unit & integration tests.

---

*File created from TypeScript source: `ts/Distributed_Database_Implementation_ts.md` — keep the TS file as the source of truth for logic and tests.*
