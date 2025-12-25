package pqc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/cloudflare/circl/kem"
)

// EncryptionManager manages PQC encryption for sensitive data
type EncryptionManager struct {
	mu        sync.RWMutex
	masterKey *PQCKeyPair
	keyCache  map[string]*PQCKeyPair // Cache for loaded keys
}

// NewEncryptionManager creates a new encryption manager
func NewEncryptionManager() *EncryptionManager {
	return &EncryptionManager{
		keyCache: make(map[string]*PQCKeyPair),
	}
}

// SetMasterKey sets the master PQC key pair for encryption
func (em *EncryptionManager) SetMasterKey(keyPair *PQCKeyPair) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.masterKey = keyPair
}

// GetMasterKey returns the master key pair
func (em *EncryptionManager) GetMasterKey() *PQCKeyPair {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.masterKey
}

// EncryptData encrypts sensitive data using PQC encryption
func (em *EncryptionManager) EncryptData(plaintext []byte, keyID string) (string, error) {
	em.mu.RLock()
	keyPair, exists := em.keyCache[keyID]
	em.mu.RUnlock()

	if !exists {
		// If key not in cache, try to use master key
		if em.masterKey != nil && em.masterKey.ID == keyID {
			keyPair = em.masterKey
		} else {
			return "", fmt.Errorf("key %s not found in cache", keyID)
		}
	}

	if !keyPair.IsActive() {
		return "", fmt.Errorf("key %s is not active", keyID)
	}

	// Encrypt the data
	ciphertext, err := keyPair.Encrypt(plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt data: %w", err)
	}

	// Create encrypted payload with metadata
	payload := map[string]interface{}{
		"key_id":     keyID,
		"algorithm":  "Kyber-768+AES-256-GCM",
		"ciphertext": base64.StdEncoding.EncodeToString(ciphertext),
	}

	// Sign the payload for integrity
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	signature, err := keyPair.Sign(payloadBytes)
	if err != nil {
		return "", fmt.Errorf("failed to sign payload: %w", err)
	}

	// Create final encrypted structure
	encrypted := map[string]interface{}{
		"payload":   payload,
		"signature": base64.StdEncoding.EncodeToString(signature),
	}

	finalBytes, err := json.Marshal(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to marshal encrypted data: %w", err)
	}

	return base64.StdEncoding.EncodeToString(finalBytes), nil
}

// DecryptData decrypts data encrypted with EncryptData
func (em *EncryptionManager) DecryptData(encryptedData string) ([]byte, error) {
	// Decode the base64 encrypted data
	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted data: %w", err)
	}

	// Unmarshal the encrypted structure
	var encrypted map[string]interface{}
	if err := json.Unmarshal(encryptedBytes, &encrypted); err != nil {
		return nil, fmt.Errorf("failed to unmarshal encrypted data: %w", err)
	}

	payloadInterface, ok := encrypted["payload"]
	if !ok {
		return nil, fmt.Errorf("missing payload in encrypted data")
	}

	signatureB64, ok := encrypted["signature"].(string)
	if !ok {
		return nil, fmt.Errorf("missing signature in encrypted data")
	}

	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Extract payload
	payloadBytes, err := json.Marshal(payloadInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	keyID, ok := payload["key_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing key_id in payload")
	}

	ciphertextB64, ok := payload["ciphertext"].(string)
	if !ok {
		return nil, fmt.Errorf("missing ciphertext in payload")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	// Get the key pair
	em.mu.RLock()
	keyPair, exists := em.keyCache[keyID]
	em.mu.RUnlock()

	if !exists {
		// If key not in cache, try to use master key
		if em.masterKey != nil && em.masterKey.ID == keyID {
			keyPair = em.masterKey
		} else {
			return nil, fmt.Errorf("key %s not found in cache", keyID)
		}
	}

	if !keyPair.IsActive() {
		return nil, fmt.Errorf("key %s is not active", keyID)
	}

	// Verify signature
	if !keyPair.Verify(payloadBytes, signature) {
		return nil, fmt.Errorf("signature verification failed")
	}

	// Decrypt the data
	plaintext, err := keyPair.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return plaintext, nil
}

// CacheKey adds a key pair to the cache
func (em *EncryptionManager) CacheKey(keyID string, keyPair *PQCKeyPair) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.keyCache[keyID] = keyPair
}

// RemoveKey removes a key pair from the cache
func (em *EncryptionManager) RemoveKey(keyID string) {
	em.mu.Lock()
	defer em.mu.Unlock()
	delete(em.keyCache, keyID)
}

// GenerateDataEncryptionKey generates a new key pair for data encryption
func (em *EncryptionManager) GenerateDataEncryptionKey(name string) (*PQCKeyPair, error) {
	keyPair, err := GeneratePQCKeyPair(name, "encryption")
	if err != nil {
		return nil, err
	}

	em.CacheKey(keyPair.ID, keyPair)
	return keyPair, nil
}

// EncryptPrivateKey encrypts a private key using the master key
func (em *EncryptionManager) EncryptPrivateKey(privateKey kem.PrivateKey) (string, error) {
	if em.masterKey == nil {
		return "", fmt.Errorf("no master key set")
	}

	// Marshal the private key
	keyBytes, err := privateKey.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Encrypt with master key
	return em.EncryptData(keyBytes, em.masterKey.ID)
}

// DecryptPrivateKey decrypts a private key using the master key
func (em *EncryptionManager) DecryptPrivateKey(encryptedKey string) (kem.PrivateKey, error) {
	if em.masterKey == nil {
		return nil, fmt.Errorf("no master key set")
	}

	// Decrypt the key bytes
	keyBytes, err := em.DecryptData(encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt private key: %w", err)
	}

	// Unmarshal the private key
	return UnmarshalKyberPrivateKey(keyBytes)
}
