use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use crate::types::*;
use crate::storage::Storage;
use crate::network::Network;
use crate::collection::DistributedCollection;

/// DistributedDbOptions contains options for the distributed database
#[derive(Debug, Clone)]
pub struct DistributedDbOptions {
    pub distributed: DistributedOptions,
}

#[derive(Debug, Clone)]
pub struct DistributedOptions {
    pub enabled: bool,
    pub network_id: String,
    pub bootstrap_peers: Vec<String>,
}

/// DistributedDatabase is the main database instance
pub struct DistributedDatabase {
    network: Arc<dyn Network>,
    storage: Arc<dyn Storage>,
    collections: Arc<RwLock<HashMap<String, Arc<RwLock<DistributedCollection>>>>>,
    distributed: bool,
}

impl DistributedDatabase {
    /// NewDistributedDatabase creates a new distributed database
    pub async fn new(
        opts: DistributedDbOptions,
        storage: Arc<dyn Storage>,
        network: Arc<dyn Network>,
    ) -> Result<Self, Box<dyn std::error::Error + Send + Sync>> {
        network.initialize().await?;

        Ok(DistributedDatabase {
            network,
            storage,
            collections: Arc::new(RwLock::new(HashMap::new())),
            distributed: opts.distributed.enabled,
        })
    }

    /// Collection returns a collection by name
    pub async fn collection(&self, name: &str) -> Arc<RwLock<DistributedCollection>> {
        let mut collections = self.collections.write().await;
        collections.entry(name.to_string()).or_insert_with(|| {
            Arc::new(RwLock::new(DistributedCollection::new(
                name.to_string(),
                Arc::clone(&self.storage),
                Arc::clone(&self.network),
            )))
        }).clone()
    }

    /// CreateNetwork creates a new network
    pub async fn create_network(&self, cfg: NetworkConfig) -> Result<String, Box<dyn std::error::Error + Send + Sync>> {
        self.network.create_network(cfg).await
    }

    /// JoinNetwork joins an existing network
    pub async fn join_network(&self, network_id: &str, bootstrap_peers: Vec<String>) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        self.network.join_network(network_id, bootstrap_peers).await
    }

    /// LeaveNetwork leaves a network
    pub async fn leave_network(&self, network_id: &str) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        self.network.leave_network(network_id).await
    }

    /// Shutdown shuts down the database
    pub async fn shutdown(&self) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        self.network.shutdown().await
    }

    /// Raw returns the underlying network
    pub fn raw_network(&self) -> Arc<dyn Network> {
        Arc::clone(&self.network)
    }
}