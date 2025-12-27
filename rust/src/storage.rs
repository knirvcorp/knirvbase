use std::collections::HashMap;
use std::fs;
use std::path::Path;
use serde::{Deserialize, Serialize};
use parking_lot::RwLock;
use crate::crypto::pqc::EncryptionManager;
use crate::types::EntryType;

/// Storage interface for persistence
#[async_trait::async_trait]
pub trait Storage: Send + Sync {
    async fn insert(&self, collection: &str, doc: HashMap<String, serde_json::Value>) -> Result<(), Box<dyn std::error::Error + Send + Sync>>;
    async fn update(&self, collection: &str, id: &str, update: HashMap<String, serde_json::Value>) -> Result<(), Box<dyn std::error::Error + Send + Sync>>;
    async fn delete(&self, collection: &str, id: &str) -> Result<(), Box<dyn std::error::Error + Send + Sync>>;
    async fn find(&self, collection: &str, id: &str) -> Result<Option<HashMap<String, serde_json::Value>>, Box<dyn std::error::Error + Send + Sync>>;
    async fn find_all(&self, collection: &str) -> Result<Vec<HashMap<String, serde_json::Value>>, Box<dyn std::error::Error + Send + Sync>>;
}

/// IndexType represents the type of index
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum IndexType {
    BTree,
    GIN,
    HNSW,
}

/// Index represents a secondary index
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Index {
    pub name: String,
    pub collection: String,
    pub index_type: IndexType,
    pub fields: Vec<String>,
    pub unique: bool,
    pub partial_expr: Option<String>,
    pub options: Option<HashMap<String, serde_json::Value>>,
}

/// FileStorage implements Storage using files
pub struct FileStorage {
    base_dir: String,
    encryption_mgr: RwLock<EncryptionManager>,
}

impl FileStorage {
    /// NewFileStorage creates a new file-based storage
    pub fn new(base_dir: String) -> Result<Self, Box<dyn std::error::Error + Send + Sync>> {
        fs::create_dir_all(&base_dir)?;

        Ok(FileStorage {
            base_dir,
            encryption_mgr: RwLock::new(EncryptionManager::new()),
        })
    }

    fn get_collection_dir(&self, collection: &str) -> String {
        Path::new(&self.base_dir).join(collection).to_string_lossy().to_string()
    }

    fn get_doc_path(&self, collection: &str, id: &str) -> String {
        Path::new(&self.get_collection_dir(collection)).join(format!("{}.json", id)).to_string_lossy().to_string()
    }

    /// SetMasterKey sets the master PQC key for encryption
    pub fn set_master_key(&self, key_pair: crate::crypto::pqc::PQCKeyPair) {
        self.encryption_mgr.write().set_master_key(key_pair);
    }

    /// IsEncryptedCollection checks if a collection contains sensitive data
    pub fn is_encrypted_collection(&self, collection: &str) -> bool {
        let sensitive_collections = [
            "credentials",
            "pqc_keys",
            "sessions",
            "audit_log",
            "threat_events",
            "access_control",
        ];

        sensitive_collections.contains(&collection)
    }

    fn deep_copy_doc(&self, doc: &HashMap<String, serde_json::Value>) -> HashMap<String, serde_json::Value> {
        serde_json::from_value(serde_json::to_value(doc).unwrap()).unwrap()
    }

    fn save_blob(&self, collection: &str, id: &str, blob: &serde_json::Value) -> Result<String, Box<dyn std::error::Error + Send + Sync>> {
        let blob_dir = Path::new(&self.get_collection_dir(collection)).join("blobs");
        fs::create_dir_all(&blob_dir)?;

        let blob_path = blob_dir.join(id);
        let data = serde_json::to_vec(blob)?;
        fs::write(blob_path, data)?;

        Ok(blob_dir.join(id).to_string_lossy().to_string())
    }

    fn load_blob(&self, blob_ref: &str) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync>> {
        let data = fs::read(blob_ref)?;
        Ok(serde_json::from_slice(&data)?)
    }

    fn encrypt_document(&self, collection: &str, doc: &mut HashMap<String, serde_json::Value>) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        let master_key = self.encryption_mgr.read().get_master_key();
        if master_key.is_none() {
            return Err("no master key set for encryption".into());
        }

        if let Some(serde_json::Value::Object(payload)) = doc.get_mut("payload") {
            let encrypted_payload = self.encrypt_payload(collection, payload, &master_key.as_ref().unwrap().id)?;
            doc.insert("payload".to_string(), encrypted_payload);
            doc.insert("encrypted".to_string(), serde_json::Value::Bool(true));
            doc.insert("encryption_key_id".to_string(), serde_json::Value::String(master_key.unwrap().id));
        }

        Ok(())
    }

    fn encrypt_payload(&self, collection: &str, payload: &serde_json::Map<String, serde_json::Value>, key_id: &str) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync>> {
        let mut encrypted = serde_json::Map::new();

        for (key, value) in payload {
            if self.is_sensitive_field(collection, key) {
                let value_bytes = serde_json::to_vec(value)?;
                let encrypted_value = self.encryption_mgr.read().encrypt_data(&value_bytes, key_id)?;
                encrypted.insert(key.clone(), serde_json::Value::String(encrypted_value));
                encrypted.insert(format!("{}_encrypted", key), serde_json::Value::Bool(true));
            } else {
                encrypted.insert(key.clone(), value.clone());
            }
        }

        Ok(serde_json::Value::Object(encrypted))
    }

    fn is_sensitive_field(&self, collection: &str, field_name: &str) -> bool {
        let sensitive_fields: HashMap<&str, Vec<&str>> = [
            ("credentials", vec!["hash", "salt"]),
            ("pqc_keys", vec!["kyber_private_key", "dilithium_private_key"]),
            ("sessions", vec!["token_hash"]),
            ("audit_log", vec!["details"]),
            ("threat_events", vec!["indicators"]),
            ("access_control", vec!["permissions"]),
        ].into_iter().collect();

        if let Some(fields) = sensitive_fields.get(collection) {
            fields.contains(&field_name)
        } else {
            false
        }
    }

    fn decrypt_document(&self, doc: &mut HashMap<String, serde_json::Value>) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        if let Some(serde_json::Value::Object(payload)) = doc.get_mut("payload") {
            let decrypted_payload = self.decrypt_payload(payload)?;
            doc.insert("payload".to_string(), decrypted_payload);
        }

        doc.remove("encrypted");
        doc.remove("encryption_key_id");

        Ok(())
    }

    fn decrypt_payload(&self, payload: &serde_json::Map<String, serde_json::Value>) -> Result<serde_json::Value, Box<dyn std::error::Error + Send + Sync>> {
        let mut decrypted = serde_json::Map::new();

        for (key, value) in payload {
            if key.ends_with("_encrypted") {
                continue;
            }

            if let Some(serde_json::Value::Bool(true)) = payload.get(&format!("{}_encrypted", key)) {
                if let serde_json::Value::String(encrypted_value) = value {
                    let decrypted_bytes = self.encryption_mgr.read().decrypt_data(encrypted_value)?;
                    let decrypted_value: serde_json::Value = serde_json::from_slice(&decrypted_bytes)?;
                    decrypted.insert(key.clone(), decrypted_value);
                }
            } else {
                decrypted.insert(key.clone(), value.clone());
            }
        }

        Ok(serde_json::Value::Object(decrypted))
    }
}

#[async_trait::async_trait]
impl Storage for FileStorage {
    async fn insert(&self, collection: &str, mut doc: HashMap<String, serde_json::Value>) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        fs::create_dir_all(&self.get_collection_dir(collection))?;

        let id = doc.get("id").and_then(|v| v.as_str()).ok_or("document must have an 'id' field")?;
        let path = self.get_doc_path(collection, id);

        let mut doc_copy = self.deep_copy_doc(&doc);

        // Handle MEMORY blob
        if let Some(serde_json::Value::String(entry_type)) = doc_copy.get("entryType") {
            if entry_type == "MEMORY" {
                if let Some(serde_json::Value::Object(payload)) = doc_copy.get_mut("payload") {
                    if let Some(blob) = payload.remove("blob") {
                        let blob_path = self.save_blob(collection, id, &blob)?;
                        payload.insert("blobRef".to_string(), serde_json::Value::String(blob_path));
                    }
                }
            }
        }

        let mut final_doc = doc_copy;

        // Encrypt sensitive collections
        if self.is_encrypted_collection(collection) {
            self.encrypt_document(collection, &mut final_doc)?;
        }

        let data = serde_json::to_vec_pretty(&final_doc)?;
        fs::write(path, data)?;

        Ok(())
    }

    async fn update(&self, collection: &str, id: &str, update: HashMap<String, serde_json::Value>) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        if let Some(mut doc) = self.find(collection, id).await? {
            for (k, v) in update {
                doc.insert(k, v);
            }
            self.insert(collection, doc).await
        } else {
            Err("document not found".into())
        }
    }

    async fn delete(&self, collection: &str, id: &str) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        let path = self.get_doc_path(collection, id);
        if Path::new(&path).exists() {
            fs::remove_file(path)?;
        }

        // Remove blob if exists
        let blob_dir = Path::new(&self.get_collection_dir(collection)).join("blobs");
        let blob_path = blob_dir.join(id);
        if blob_path.exists() {
            fs::remove_file(blob_path)?;
        }

        Ok(())
    }

    async fn find(&self, collection: &str, id: &str) -> Result<Option<HashMap<String, serde_json::Value>>, Box<dyn std::error::Error + Send + Sync>> {
        let path = self.get_doc_path(collection, id);

        if !Path::new(&path).exists() {
            return Ok(None);
        }

        let data = fs::read(&path)?;
        let mut doc: HashMap<String, serde_json::Value> = serde_json::from_slice(&data)?;

        // Decrypt if document is encrypted
        if let Some(serde_json::Value::Bool(true)) = doc.get("encrypted") {
            self.decrypt_document(&mut doc)?;
        }

        // Load blob for MEMORY
        if let Some(serde_json::Value::String(entry_type)) = doc.get("entryType") {
            if entry_type == "MEMORY" {
                if let Some(serde_json::Value::Object(payload)) = doc.get_mut("payload") {
                    if let Some(serde_json::Value::String(blob_ref)) = payload.get("blobRef") {
                        if let Ok(blob) = self.load_blob(blob_ref) {
                            payload.insert("blob".to_string(), blob);
                            payload.remove("blobRef");
                        }
                    }
                }
            }
        }

        Ok(Some(doc))
    }

    async fn find_all(&self, collection: &str) -> Result<Vec<HashMap<String, serde_json::Value>>, Box<dyn std::error::Error + Send + Sync>> {
        let dir = self.get_collection_dir(collection);
        let mut docs = Vec::new();

        if !Path::new(&dir).exists() {
            return Ok(docs);
        }

        for entry in fs::read_dir(&dir)? {
            let entry = entry?;
            let path = entry.path();

            if path.extension().and_then(|s| s.to_str()) == Some("json") {
                if let Some(file_stem) = path.file_stem().and_then(|s| s.to_str()) {
                    if let Some(doc) = self.find(collection, file_stem).await? {
                        docs.push(doc);
                    }
                }
            }
        }

        Ok(docs)
    }
}