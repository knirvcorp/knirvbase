use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use chrono::{DateTime, Utc, Duration};
use crate::clock::VectorClock;

/// EntryType specifies the kind of data stored.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum EntryType {
    Memory,
    Auth,
}

impl From<&str> for EntryType {
    fn from(s: &str) -> Self {
        match s {
            "MEMORY" => EntryType::Memory,
            "AUTH" => EntryType::Auth,
            _ => EntryType::Memory, // default
        }
    }
}

impl std::fmt::Display for EntryType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            EntryType::Memory => write!(f, "MEMORY"),
            EntryType::Auth => write!(f, "AUTH"),
        }
    }
}

impl std::str::FromStr for EntryType {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s {
            "MEMORY" => Ok(EntryType::Memory),
            "AUTH" => Ok(EntryType::Auth),
            _ => Err(format!("Unknown entry type: {}", s)),
        }
    }
}

/// DistributedDocument augments a document with CRDT metadata
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct DistributedDocument {
    pub id: String,
    pub entry_type: EntryType,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub payload: Option<HashMap<String, serde_json::Value>>,
    pub vector: VectorClock,
    pub timestamp: i64,
    pub peer_id: String,
    /// Stage is an optional marker used to indicate special handling for a document.
    /// Supported values: "post-pending" (document is staged and will be posted as a KNIRVGRAPH
    /// transaction during the next sync), or empty string for normal documents.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stage: Option<String>,
    #[serde(skip_serializing_if = "std::ops::Not::not")]
    pub deleted: bool,
}

/// OperationType enumerates CRDT operation kinds
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum OperationType {
    Insert,
    Update,
    Delete,
}

/// CRDTOperation represents a change to be synchronized
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct CRDTOperation {
    pub id: String,
    pub op_type: OperationType,
    pub collection: String,
    pub document_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<DistributedDocument>,
    pub vector: VectorClock,
    pub timestamp: i64,
    pub peer_id: String,
}

/// NetworkConfig holds network-level configuration
/// Additional fields control posting/staging behavior:
///   - DefaultPostingNetwork: the network to which staged entries are posted (e.g. "knirvgraph").
///   - AutoPostClassifications: a list of EntryTypes that are automatically staged for posting by classification.
///   - PrivateByDefault: when true (default), entries are private unless staged or explicitly configured.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct NetworkConfig {
    pub network_id: String,
    pub name: String,
    pub collections: HashMap<String, bool>,
    pub bootstrap_peers: Vec<String>,
    /// Default posting target for staged entries (e.g., "knirvgraph").
    pub default_posting_network: String,
    /// Entry classifications which are auto-staged for posting. Common defaults include
    /// EntryType values like "ERROR", "CONTEXT", and "IDEA".
    pub auto_post_classifications: Vec<EntryType>,
    /// Entries are private by default unless staged or configured otherwise.
    pub private_by_default: bool,
    pub encryption: EncryptionConfig,
    pub replication: ReplicationConfig,
    pub discovery: DiscoveryConfig,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, Default)]
pub struct EncryptionConfig {
    pub enabled: bool,
    pub shared_secret: String,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, Default)]
pub struct ReplicationConfig {
    pub factor: i32,
    pub strategy: String, // full | partial | leader
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, Default)]
pub struct DiscoveryConfig {
    pub mdns: bool,
    pub bootstrap: bool,
}

/// PeerInfo
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct PeerInfo {
    pub peer_id: String,
    pub addrs: Vec<String>,
    pub protocols: Vec<String>,
    pub latency: Duration,
    pub last_seen: DateTime<Utc>,
    pub collections: Vec<String>,
}

/// SyncState for a collection/network
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SyncState {
    pub collection: String,
    pub network_id: String,
    pub local_vector: VectorClock,
    pub last_sync: DateTime<Utc>,
    pub pending_operations: Vec<CRDTOperation>,
    /// StagedEntries contains IDs of documents marked with `_stage == "post-pending"`.
    /// These will be converted to KNIRVGRAPH transactions and posted during the next sync.
    pub staged_entries: Vec<String>,
    pub sync_in_progress: bool,
}

/// NetworkStats
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct NetworkStats {
    pub network_id: String,
    pub connected_peers: i32,
    pub total_peers: i32,
    pub collections_shared: i32,
    pub operations_sent: i64,
    pub operations_received: i64,
    pub bytes_transferred: i64,
    pub average_latency: Duration,
}

/// MessageType strings for protocol
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum MessageType {
    SyncRequest,
    SyncResponse,
    Operation,
    Heartbeat,
    CollectionAnnounce,
    CollectionRequest,
}

impl std::fmt::Display for MessageType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            MessageType::SyncRequest => write!(f, "sync_request"),
            MessageType::SyncResponse => write!(f, "sync_response"),
            MessageType::Operation => write!(f, "operation"),
            MessageType::Heartbeat => write!(f, "heartbeat"),
            MessageType::CollectionAnnounce => write!(f, "collection_announce"),
            MessageType::CollectionRequest => write!(f, "collection_request"),
        }
    }
}

/// ProtocolMessage generic envelope
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ProtocolMessage {
    pub msg_type: MessageType,
    pub network_id: String,
    pub sender_id: String,
    pub timestamp: i64,
    pub payload: serde_json::Value,
}