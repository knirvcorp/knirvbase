use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

pub mod clock;
pub mod types;
pub mod crypto;
pub mod storage;
pub mod network;
pub mod resolver;
pub mod collection;
pub mod database;
pub mod query;

// Re-export commonly used types
pub use clock::VectorClock;
pub use types::*;
pub use crypto::pqc::*;
pub use storage::*;
pub use network::*;
pub use resolver::*;
pub use collection::*;
pub use database::*;
pub use query::*;

/// Options contains configuration for the library
#[derive(Debug, Clone)]
pub struct Options {
    pub data_dir: String,
    pub distributed_enabled: bool,
    pub distributed_network_id: String,
    pub distributed_bootstrap_peers: Vec<String>,
}

/// DB is the public wrapper around the internal DistributedDatabase
pub struct DB {
    db: DistributedDatabase,
    storage: Arc<dyn Storage>,
}

impl DB {
    /// New constructs a DB instance with the provided options
    pub async fn new(opts: Options) -> Result<Self, Box<dyn std::error::Error + Send + Sync>> {
        if opts.data_dir.is_empty() {
            return Err("DataDir cannot be empty".into());
        }

        let storage: Arc<dyn Storage> = Arc::new(FileStorage::new(opts.data_dir)?);
        let network = Arc::new(NetworkManager::new());

        let db_opts = DistributedDbOptions {
            distributed: DistributedOptions {
                enabled: opts.distributed_enabled,
                network_id: opts.distributed_network_id,
                bootstrap_peers: opts.distributed_bootstrap_peers,
            },
        };

        let db = DistributedDatabase::new(db_opts, Arc::clone(&storage), network).await?;

        Ok(DB { db, storage })
    }

    /// CreateNetwork creates a network using the underlying manager
    pub async fn create_network(&self, cfg: NetworkConfig) -> Result<String, Box<dyn std::error::Error + Send + Sync>> {
        self.db.create_network(cfg).await
    }

    /// Collection returns a collection interface for use by callers
    pub async fn collection(&self, name: &str) -> Result<CollectionAdapter, Box<dyn std::error::Error + Send + Sync>> {
        if name.is_empty() {
            return Err("collection name cannot be empty".into());
        }

        let coll = self.db.collection(name).await;
        Ok(CollectionAdapter { c: coll })
    }

    /// Raw returns the underlying internal DistributedDatabase for advanced usage
    pub fn raw(&self) -> &DistributedDatabase {
        &self.db
    }

    /// Shutdown stops the underlying network manager
    pub async fn shutdown(self) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        self.db.shutdown().await
    }
}

/// Collection is a thin interface representing collection operations consumers need
#[async_trait::async_trait]
pub trait Collection {
    async fn insert(&self, doc: HashMap<String, serde_json::Value>) -> Result<HashMap<String, serde_json::Value>, Box<dyn std::error::Error + Send + Sync>>;
    async fn update(&self, id: &str, update: HashMap<String, serde_json::Value>) -> Result<i32, Box<dyn std::error::Error + Send + Sync>>;
    async fn delete(&self, id: &str) -> Result<i32, Box<dyn std::error::Error + Send + Sync>>;
    async fn find(&self, id: &str) -> Result<Option<HashMap<String, serde_json::Value>>, Box<dyn std::error::Error + Send + Sync>>;
    async fn find_all(&self) -> Result<Vec<HashMap<String, serde_json::Value>>, Box<dyn std::error::Error + Send + Sync>>;
    async fn attach_to_network(&self, network_id: &str) -> Result<(), Box<dyn std::error::Error + Send + Sync>>;
    async fn detach_from_network(&self) -> Result<(), Box<dyn std::error::Error + Send + Sync>>;
    async fn force_sync(&self) -> Result<(), Box<dyn std::error::Error + Send + Sync>>;
}

/// CollectionAdapter adapts internal DistributedCollection to the Collection interface
pub struct CollectionAdapter {
    c: Arc<RwLock<DistributedCollection>>,
}

#[async_trait::async_trait]
impl Collection for CollectionAdapter {
    async fn insert(&self, doc: HashMap<String, serde_json::Value>) -> Result<HashMap<String, serde_json::Value>, Box<dyn std::error::Error + Send + Sync>> {
        let c = self.c.read().await;
        c.insert("", doc).await
    }

    async fn update(&self, id: &str, update: HashMap<String, serde_json::Value>) -> Result<i32, Box<dyn std::error::Error + Send + Sync>> {
        let c = self.c.read().await;
        c.update(id, update).await
    }

    async fn delete(&self, id: &str) -> Result<i32, Box<dyn std::error::Error + Send + Sync>> {
        let c = self.c.read().await;
        c.delete(id).await
    }

    async fn find(&self, id: &str) -> Result<Option<HashMap<String, serde_json::Value>>, Box<dyn std::error::Error + Send + Sync>> {
        let c = self.c.read().await;
        c.find(id).await
    }

    async fn find_all(&self) -> Result<Vec<HashMap<String, serde_json::Value>>, Box<dyn std::error::Error + Send + Sync>> {
        let c = self.c.read().await;
        c.find_all().await
    }

    async fn attach_to_network(&self, network_id: &str) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        let mut c = self.c.write().await;
        c.attach_to_network(network_id).await
    }

    async fn detach_from_network(&self) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        let mut c = self.c.write().await;
        c.detach_from_network().await
    }

    async fn force_sync(&self) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        let c = self.c.read().await;
        c.force_sync().await
    }
}