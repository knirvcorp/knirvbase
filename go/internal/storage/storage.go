package distributed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/knirv/knirvbase/internal/crypto/pqc"
	"github.com/knirv/knirvbase/internal/types"
)

// Storage interface for persistence
type Storage interface {
	Insert(collection string, doc map[string]interface{}) error
	Update(collection, id string, update map[string]interface{}) error
	Delete(collection, id string) error
	Find(collection, id string) (map[string]interface{}, error)
	FindAll(collection string) ([]map[string]interface{}, error)

	// Index management
	CreateIndex(collection, name string, indexType IndexType, fields []string, unique bool, partialExpr string, options map[string]interface{}) error
	DropIndex(collection, name string) error
	GetIndex(collection, name string) *Index
	GetIndexesForCollection(collection string) []*Index
	QueryIndex(collection, indexName string, query map[string]interface{}) ([]string, error)
}

// FileStorage implements Storage using files
type FileStorage struct {
	baseDir       string
	indexManager  *IndexManager
	encryptionMgr *pqc.EncryptionManager
	mu            sync.RWMutex
}

func NewFileStorage(baseDir string) *FileStorage {
	os.MkdirAll(baseDir, 0755)
	indexManager := NewIndexManager(baseDir)
	indexManager.LoadIndexes() // Load existing indexes
	return &FileStorage{
		baseDir:       baseDir,
		indexManager:  indexManager,
		encryptionMgr: pqc.NewEncryptionManager(),
	}
}

func (fs *FileStorage) getCollectionDir(collection string) string {
	return filepath.Join(fs.baseDir, collection)
}

func (fs *FileStorage) getDocPath(collection, id string) string {
	return filepath.Join(fs.getCollectionDir(collection), id+".json")
}

// SetMasterKey sets the master PQC key for encryption
func (fs *FileStorage) SetMasterKey(keyPair *pqc.PQCKeyPair) {
	fs.encryptionMgr.SetMasterKey(keyPair)
}

// IsEncryptedCollection checks if a collection contains sensitive data that should be encrypted
func (fs *FileStorage) IsEncryptedCollection(collection string) bool {
	// Collections that contain sensitive data requiring encryption at rest
	sensitiveCollections := []string{
		"credentials",    // hash, salt fields
		"pqc_keys",       // private key fields
		"sessions",       // token_hash field
		"audit_log",      // details field
		"threat_events",  // indicators field
		"access_control", // permissions field
	}
	for _, sc := range sensitiveCollections {
		if collection == sc {
			return true
		}
	}
	return false
}

func (fs *FileStorage) Insert(collection string, doc map[string]interface{}) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	os.MkdirAll(fs.getCollectionDir(collection), 0755)
	path := fs.getDocPath(collection, doc["id"].(string))

	// Create a copy for processing
	docCopy := fs.deepCopyDoc(doc)

	// Handle MEMORY blob
	if entryType, ok := docCopy["entryType"].(types.EntryType); ok && entryType == types.EntryTypeMemory {
		if payload, ok := docCopy["payload"].(map[string]interface{}); ok {
			if blob, hasBlob := payload["blob"]; hasBlob {
				blobPath := fs.saveBlob(collection, docCopy["id"].(string), blob)
				payload["blobRef"] = blobPath
				delete(payload, "blob")
				docCopy["payload"] = payload
			}
		}
	}

	// Encrypt sensitive collections (only if master key is set)
	if fs.IsEncryptedCollection(collection) && fs.encryptionMgr.GetMasterKey() != nil {
		encryptedDoc, err := fs.encryptDocument(collection, docCopy)
		if err != nil {
			return fmt.Errorf("failed to encrypt document: %w", err)
		}
		docCopy = encryptedDoc
	}

	data, err := json.Marshal(docCopy)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return err
	}

	// Update indexes (use original doc for indexing)
	return fs.indexManager.Insert(collection, doc)
}

func (fs *FileStorage) Update(collection, id string, update map[string]interface{}) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	doc, err := fs.Find(collection, id)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("not found")
	}

	for k, v := range update {
		doc[k] = v
	}

	return fs.Insert(collection, doc)
}

func (fs *FileStorage) Delete(collection, id string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.getDocPath(collection, id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Remove blob if exists
	blobDir := filepath.Join(fs.getCollectionDir(collection), "blobs")
	blobPath := filepath.Join(blobDir, id)
	os.Remove(blobPath)

	// Remove from indexes
	return fs.indexManager.Delete(collection, id)
}

func (fs *FileStorage) Find(collection, id string) (map[string]interface{}, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := fs.getDocPath(collection, id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	// Decrypt if document is encrypted and we have a master key
	if encrypted, ok := doc["encrypted"].(bool); ok && encrypted && fs.encryptionMgr.GetMasterKey() != nil {
		decryptedDoc, err := fs.decryptDocument(doc)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt document: %w", err)
		}
		doc = decryptedDoc
	}

	// Load blob for MEMORY
	if entryType, ok := doc["entryType"].(string); ok && entryType == string(types.EntryTypeMemory) {
		if payload, ok := doc["payload"].(map[string]interface{}); ok {
			if blobRef, hasRef := payload["blobRef"].(string); hasRef {
				blob, err := fs.loadBlob(blobRef)
				if err == nil {
					payload["blob"] = blob
					delete(payload, "blobRef")
				}
			}
		}
	}

	return doc, nil
}

func (fs *FileStorage) FindAll(collection string) ([]map[string]interface{}, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	dir := fs.getCollectionDir(collection)
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []map[string]interface{}{}, nil
		}
		return nil, err
	}

	var docs []map[string]interface{}
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			id := file.Name()[:len(file.Name())-5]
			doc, err := fs.Find(collection, id)
			if err != nil {
				continue
			}
			if doc != nil {
				docs = append(docs, doc)
			}
		}
	}
	return docs, nil
}

func (fs *FileStorage) saveBlob(collection, id string, blob interface{}) string {
	blobDir := filepath.Join(fs.getCollectionDir(collection), "blobs")
	os.MkdirAll(blobDir, 0755)

	blobPath := filepath.Join(blobDir, id)
	data, _ := json.Marshal(blob)
	os.WriteFile(blobPath, data, 0644)
	return blobPath
}

func (fs *FileStorage) loadBlob(blobRef string) (interface{}, error) {
	data, err := os.ReadFile(blobRef)
	if err != nil {
		return nil, err
	}
	var blob interface{}
	json.Unmarshal(data, &blob)
	return blob, nil
}

// Index management methods

func (fs *FileStorage) CreateIndex(collection, name string, indexType IndexType, fields []string, unique bool, partialExpr string, options map[string]interface{}) error {
	return fs.indexManager.CreateIndex(collection, name, indexType, fields, unique, partialExpr, options)
}

func (fs *FileStorage) DropIndex(collection, name string) error {
	return fs.indexManager.DropIndex(collection, name)
}

func (fs *FileStorage) GetIndex(collection, name string) *Index {
	return fs.indexManager.GetIndex(collection, name)
}

func (fs *FileStorage) GetIndexesForCollection(collection string) []*Index {
	return fs.indexManager.GetIndexesForCollection(collection)
}

func (fs *FileStorage) QueryIndex(collection, indexName string, query map[string]interface{}) ([]string, error) {
	return fs.indexManager.QueryIndex(collection, indexName, query)
}

// deepCopyDoc creates a deep copy of a document
func (fs *FileStorage) deepCopyDoc(doc map[string]interface{}) map[string]interface{} {
	data, _ := json.Marshal(doc)
	var copy map[string]interface{}
	json.Unmarshal(data, &copy)
	return copy
}

// encryptDocument encrypts sensitive fields in a document
func (fs *FileStorage) encryptDocument(collection string, doc map[string]interface{}) (map[string]interface{}, error) {
	masterKey := fs.encryptionMgr.GetMasterKey()
	if masterKey == nil {
		return nil, fmt.Errorf("no master key set for encryption")
	}

	// Encrypt the payload
	if payload, ok := doc["payload"].(map[string]interface{}); ok {
		encryptedPayload, err := fs.encryptPayload(collection, payload, masterKey.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt payload: %w", err)
		}
		doc["payload"] = encryptedPayload
		doc["encrypted"] = true
		doc["encryption_key_id"] = masterKey.ID
	}

	return doc, nil
}

// encryptPayload encrypts sensitive fields in the payload
func (fs *FileStorage) encryptPayload(collection string, payload map[string]interface{}, keyID string) (map[string]interface{}, error) {
	encrypted := make(map[string]interface{})

	for key, value := range payload {
		// Encrypt sensitive fields based on collection and field name
		if fs.isSensitiveField(collection, key) {
			// Convert value to bytes for encryption
			valueBytes, err := json.Marshal(value)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal field %s: %w", key, err)
			}

			encryptedValue, err := fs.encryptionMgr.EncryptData(valueBytes, keyID)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt field %s: %w", key, err)
			}

			encrypted[key] = encryptedValue
			encrypted[key+"_encrypted"] = true
		} else {
			encrypted[key] = value
		}
	}

	return encrypted, nil
}

// isSensitiveField checks if a field contains sensitive data that should be encrypted
// Based on the ASIC-Shield security specification
func (fs *FileStorage) isSensitiveField(collection, fieldName string) bool {
	// Define sensitive fields by collection (field-level encryption)
	sensitiveFields := map[string][]string{
		"credentials": {
			"hash", // KDF output
			"salt", // Random salt for KDF
		},
		"pqc_keys": {
			"kyber_private_key",     // Kyber private key
			"dilithium_private_key", // Dilithium private key
		},
		"sessions": {
			"token_hash", // Session token hash
		},
		"audit_log": {
			"details", // Event details that may contain sensitive info
		},
		"threat_events": {
			"indicators", // Attack indicators and evidence
		},
		"access_control": {
			"permissions", // Access control permissions
		},
	}

	if fields, exists := sensitiveFields[collection]; exists {
		for _, field := range fields {
			if fieldName == field {
				return true
			}
		}
	}

	return false
}

// decryptDocument decrypts an encrypted document
func (fs *FileStorage) decryptDocument(doc map[string]interface{}) (map[string]interface{}, error) {
	keyID, ok := doc["encryption_key_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing encryption_key_id")
	}

	// Decrypt the payload
	if payload, ok := doc["payload"].(map[string]interface{}); ok {
		decryptedPayload, err := fs.decryptPayload(payload, keyID)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt payload: %w", err)
		}
		doc["payload"] = decryptedPayload
	}

	// Remove encryption metadata
	delete(doc, "encrypted")
	delete(doc, "encryption_key_id")

	return doc, nil
}

// decryptPayload decrypts encrypted fields in the payload
func (fs *FileStorage) decryptPayload(payload map[string]interface{}, _ string) (map[string]interface{}, error) {
	decrypted := make(map[string]interface{})

	for key, value := range payload {
		// Check if field is encrypted
		if strings.HasSuffix(key, "_encrypted") {
			// This is an encryption marker, skip
			continue
		}

		if isEncrypted, ok := payload[key+"_encrypted"].(bool); ok && isEncrypted {
			// This field is encrypted
			encryptedValue, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("encrypted field %s is not a string", key)
			}

			decryptedBytes, err := fs.encryptionMgr.DecryptData(encryptedValue)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt field %s: %w", key, err)
			}

			// Unmarshal the decrypted value
			var decryptedValue interface{}
			if err := json.Unmarshal(decryptedBytes, &decryptedValue); err != nil {
				return nil, fmt.Errorf("failed to unmarshal decrypted field %s: %w", key, err)
			}

			decrypted[key] = decryptedValue
		} else {
			decrypted[key] = value
		}
	}

	return decrypted, nil
}
