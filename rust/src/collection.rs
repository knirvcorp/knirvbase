use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use crate::types::*;
use crate::storage::Storage;
use crate::network::Network;
use crate::resolver::CRDTResolver;

/// Collection represents a distributed collection of documents
pub struct DistributedCollection {
    name: String,
    storage: Arc<dyn Storage>,
    network: Arc<dyn Network>,
    network_id: Option<String>,
    resolver: CRDTResolver,
    sync_state: Arc<RwLock<SyncState>>,
}

impl DistributedCollection {
    /// NewDistributedCollection creates a new distributed collection
    pub fn new(
        name: String,
        storage: Arc<dyn Storage>,
        network: Arc<dyn Network>,
    ) -> Self {
        DistributedCollection {
            name: name.clone(),
            storage,
            network,
            network_id: None,
            resolver: CRDTResolver::new(),
            sync_state: Arc::new(RwLock::new(SyncState {
                collection: name.clone(),
                network_id: "".to_string(),
                local_vector: crate::clock::VectorClock::new(),
                last_sync: chrono::Utc::now(),
                pending_operations: vec![],
                staged_entries: vec![],
                sync_in_progress: false,
            })),
        }
    }

    /// Insert inserts a document into the collection
    pub async fn insert(&self, ctx: &str, doc: HashMap<String, serde_json::Value>) -> Result<HashMap<String, serde_json::Value>, Box<dyn std::error::Error + Send + Sync>> {
        // Add CRDT metadata
        let mut distributed_doc = DistributedDocument {
            id: doc.get("id").and_then(|v| v.as_str()).unwrap_or("").to_string(),
            entry_type: EntryType::Memory, // Default
            payload: Some(doc.clone()),
            vector: crate::clock::VectorClock::new().increment(self.network.get_peer_id()),
            timestamp: chrono::Utc::now().timestamp(),
            peer_id: self.network.get_peer_id(),
            stage: None,
            deleted: false,
        };

        // Store locally
        let storage_doc = serde_json::to_value(&distributed_doc)?;
        let mut storage_map = serde_json::from_value::<HashMap<String, serde_json::Value>>(storage_doc)?;
        self.storage.insert(&self.name, storage_map).await?;

        // Emit CRDT operation if networked
        if let Some(network_id) = &self.network_id {
            let operation = CRDTOperation {
                id: uuid::Uuid::new_v4().to_string(),
                op_type: OperationType::Insert,
                collection: self.name.clone(),
                document_id: distributed_doc.id.clone(),
                data: Some(distributed_doc),
                vector: crate::clock::VectorClock::new().increment(self.network.get_peer_id()),
                timestamp: chrono::Utc::now().timestamp(),
                peer_id: self.network.get_peer_id(),
            };

            let msg = ProtocolMessage {
                msg_type: MessageType::Operation,
                network_id: network_id.clone(),
                sender_id: self.network.get_peer_id(),
                timestamp: chrono::Utc::now().timestamp(),
                payload: serde_json::to_value(&operation)?,
            };

            self.network.broadcast_message(network_id, msg).await?;
        }

        Ok(doc)
    }

    /// Update updates a document in the collection
    pub async fn update(&self, id: &str, update: HashMap<String, serde_json::Value>) -> Result<i32, Box<dyn std::error::Error + Send + Sync>> {
        if let Some(mut doc) = self.storage.find(&self.name, id).await? {
            for (k, v) in update {
                doc.insert(k, v);
            }
            self.storage.insert(&self.name, doc).await?;
            Ok(1)
        } else {
            Ok(0)
        }
    }

    /// Delete deletes a document from the collection
    pub async fn delete(&self, id: &str) -> Result<i32, Box<dyn std::error::Error + Send + Sync>> {
        self.storage.delete(&self.name, id).await?;
        Ok(1)
    }

    /// Find finds a document by ID
    pub async fn find(&self, id: &str) -> Result<Option<HashMap<String, serde_json::Value>>, Box<dyn std::error::Error + Send + Sync>> {
        self.storage.find(&self.name, id).await
    }

    /// FindAll finds all documents in the collection
    pub async fn find_all(&self) -> Result<Vec<HashMap<String, serde_json::Value>>, Box<dyn std::error::Error + Send + Sync>> {
        self.storage.find_all(&self.name).await
    }

    /// AttachToNetwork attaches the collection to a network
    pub async fn attach_to_network(&mut self, network_id: &str) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        self.network_id = Some(network_id.to_string());
        self.network.add_collection_to_network(network_id, &self.name).await?;

        let mut sync_state = self.sync_state.write().await;
        sync_state.network_id = network_id.to_string();

        Ok(())
    }

    /// DetachFromNetwork detaches the collection from its network
    pub async fn detach_from_network(&mut self) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        if let Some(network_id) = &self.network_id {
            self.network.remove_collection_from_network(network_id, &self.name).await?;
        }
        self.network_id = None;
        Ok(())
    }

    /// ForceSync forces a synchronization of the collection
    pub async fn force_sync(&self) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        // Simplified sync implementation
        let mut sync_state = self.sync_state.write().await;
        sync_state.last_sync = chrono::Utc::now();
        sync_state.sync_in_progress = false;
        Ok(())
    }
}