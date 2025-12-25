package distributed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/knirv/knirvbase/internal/crypto/pqc"
)

func TestFileStorage_PQCEncryption(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "knirvbase_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create storage
	storage := NewFileStorage(tmpDir)

	// Generate master key
	masterKey, err := pqc.GeneratePQCKeyPair("master", "encryption")
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	storage.SetMasterKey(masterKey)
	storage.encryptionMgr.CacheKey(masterKey.ID, masterKey) // Explicitly cache the key
	t.Logf("Master key ID: %s", masterKey.ID)

	// Create a document with sensitive data
	doc := map[string]interface{}{
		"id":        "test-cred-1",
		"entryType": "CREDENTIAL",
		"payload": map[string]interface{}{
			"username": "alice@example.com",
			"hash":     "sensitive_hash_data",
			"salt":     "sensitive_salt_data",
			"email":    "alice@example.com", // not sensitive
		},
	}

	// Insert document (should be encrypted)
	err = storage.Insert("credentials", doc)
	if err != nil {
		t.Fatalf("Failed to insert document: %v", err)
	}

	// Read document back (should be decrypted)
	retrieved, err := storage.Find("credentials", "test-cred-1")
	if err != nil {
		t.Fatalf("Failed to find document: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Document not found")
	}

	// Check that sensitive fields are decrypted
	payload := retrieved["payload"].(map[string]interface{})
	if payload["hash"] != "sensitive_hash_data" {
		t.Errorf("Hash not decrypted correctly: got %v", payload["hash"])
	}

	if payload["salt"] != "sensitive_salt_data" {
		t.Errorf("Salt not decrypted correctly: got %v", payload["salt"])
	}

	if payload["email"] != "alice@example.com" {
		t.Errorf("Email not preserved: got %v", payload["email"])
	}

	// Check that the file on disk is encrypted
	docPath := filepath.Join(tmpDir, "credentials", "test-cred-1.json")
	encryptedData, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("Failed to read encrypted file: %v", err)
	}

	// Parse the encrypted document
	var encryptedDoc map[string]interface{}
	if err := json.Unmarshal(encryptedData, &encryptedDoc); err != nil {
		t.Fatalf("Failed to parse encrypted document: %v", err)
	}

	// Should have encryption metadata
	if _, ok := encryptedDoc["encrypted"]; !ok {
		t.Error("Document should be marked as encrypted")
	}

	if keyID, ok := encryptedDoc["encryption_key_id"]; ok {
		t.Logf("Document encrypted with key ID: %v", keyID)
	} else {
		t.Error("Document should have encryption key ID")
	}

	// Payload should be encrypted
	if _, ok := encryptedDoc["payload"]; !ok {
		t.Error("Encrypted document should have payload")
	}
}

func TestFileStorage_EncryptionAtRest_AllCollections(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "knirvbase_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create storage
	storage := NewFileStorage(tmpDir)

	// Generate master key
	masterKey, err := pqc.GeneratePQCKeyPair("master", "encryption")
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	storage.SetMasterKey(masterKey)
	storage.encryptionMgr.CacheKey(masterKey.ID, masterKey)

	testCases := []struct {
		collection      string
		doc             map[string]interface{}
		sensitiveFields []string
	}{
		{
			collection: "credentials",
			doc: map[string]interface{}{
				"id":        "user1",
				"entryType": "CREDENTIAL",
				"payload": map[string]interface{}{
					"username": "alice@example.com",
					"hash":     "sensitive_hash_data",
					"salt":     "sensitive_salt_data",
					"email":    "alice@example.com", // not sensitive
				},
			},
			sensitiveFields: []string{"hash", "salt"},
		},
		{
			collection: "pqc_keys",
			doc: map[string]interface{}{
				"id":        "key1",
				"entryType": "PQC_KEY",
				"payload": map[string]interface{}{
					"key_name":              "test_key",
					"kyber_public_key":      "public_key_data",
					"kyber_private_key":     "sensitive_private_key_data",
					"dilithium_private_key": "sensitive_dilithium_key_data",
				},
			},
			sensitiveFields: []string{"kyber_private_key", "dilithium_private_key"},
		},
		{
			collection: "sessions",
			doc: map[string]interface{}{
				"id":        "session1",
				"entryType": "SESSION",
				"payload": map[string]interface{}{
					"session_id": "abc123",
					"token_hash": "sensitive_token_hash",
					"ip_address": "192.168.1.1",
				},
			},
			sensitiveFields: []string{"token_hash"},
		},
		{
			collection: "audit_log",
			doc: map[string]interface{}{
				"id":        "audit1",
				"entryType": "AUDIT",
				"payload": map[string]interface{}{
					"event_type": "login",
					"username":   "alice",
					"details": map[string]interface{}{
						"ip":         "192.168.1.1",
						"user_agent": "Mozilla/5.0",
					},
				},
			},
			sensitiveFields: []string{"details"},
		},
		{
			collection: "threat_events",
			doc: map[string]interface{}{
				"id":        "threat1",
				"entryType": "THREAT",
				"payload": map[string]interface{}{
					"threat_type": "brute_force",
					"ip_address":  "10.0.0.1",
					"indicators": map[string]interface{}{
						"attempts": 100,
						"patterns": []string{"pattern1", "pattern2"},
					},
				},
			},
			sensitiveFields: []string{"indicators"},
		},
		{
			collection: "access_control",
			doc: map[string]interface{}{
				"id":        "acl1",
				"entryType": "ACCESS_CONTROL",
				"payload": map[string]interface{}{
					"credential_id": "user1",
					"role":          "admin",
					"permissions":   []string{"read", "write", "delete"},
				},
			},
			sensitiveFields: []string{"permissions"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.collection, func(t *testing.T) {
			// Insert document (should encrypt sensitive fields)
			err := storage.Insert(tc.collection, tc.doc)
			if err != nil {
				t.Fatalf("Failed to insert document: %v", err)
			}

			// Read document back (should decrypt sensitive fields)
			retrieved, err := storage.Find(tc.collection, tc.doc["id"].(string))
			if err != nil {
				t.Fatalf("Failed to find document: %v", err)
			}

			if retrieved == nil {
				t.Fatal("Document not found")
			}

			// Check that sensitive fields are decrypted correctly
			payload := retrieved["payload"].(map[string]interface{})
			for _, field := range tc.sensitiveFields {
				expectedValue := tc.doc["payload"].(map[string]interface{})[field]
				actualValue := payload[field]
				if actualValue == nil {
					t.Errorf("Field %s should be present in decrypted document", field)
					continue
				}

				// For complex types, compare as JSON strings
				expectedJSON, _ := json.Marshal(expectedValue)
				actualJSON, _ := json.Marshal(actualValue)
				if string(expectedJSON) != string(actualJSON) {
					t.Errorf("Field %s not decrypted correctly: expected %v, got %v", field, expectedValue, actualValue)
				}
			}

			// Check that non-sensitive fields are preserved
			for field, expectedValue := range tc.doc["payload"].(map[string]interface{}) {
				isSensitive := false
				for _, sf := range tc.sensitiveFields {
					if field == sf {
						isSensitive = true
						break
					}
				}
				if !isSensitive {
					actualValue := payload[field]
					if actualValue != expectedValue {
						t.Errorf("Non-sensitive field %s not preserved: expected %v, got %v", field, expectedValue, actualValue)
					}
				}
			}

			// Check that the file on disk is encrypted
			docPath := filepath.Join(tmpDir, tc.collection, tc.doc["id"].(string)+".json")
			encryptedData, err := os.ReadFile(docPath)
			if err != nil {
				t.Fatalf("Failed to read encrypted file: %v", err)
			}

			// Parse the encrypted document
			var encryptedDoc map[string]interface{}
			if err := json.Unmarshal(encryptedData, &encryptedDoc); err != nil {
				t.Fatalf("Failed to parse encrypted document: %v", err)
			}

			// Should have encryption metadata
			if _, ok := encryptedDoc["encrypted"]; !ok {
				t.Error("Document should be marked as encrypted")
			}

			if keyID, ok := encryptedDoc["encryption_key_id"]; ok {
				if keyID != masterKey.ID {
					t.Errorf("Document should be encrypted with master key, got %v", keyID)
				}
			} else {
				t.Error("Document should have encryption key ID")
			}

			// Sensitive fields in encrypted payload should be encrypted
			if encryptedPayload, ok := encryptedDoc["payload"].(map[string]interface{}); ok {
				for _, field := range tc.sensitiveFields {
					if encryptedValue, exists := encryptedPayload[field]; exists {
						// Should be a base64 string (encrypted)
						if _, ok := encryptedValue.(string); !ok {
							t.Errorf("Sensitive field %s should be encrypted as string", field)
						}
						if _, hasMarker := encryptedPayload[field+"_encrypted"].(bool); !hasMarker {
							t.Errorf("Sensitive field %s should have encryption marker", field)
						}
					}
				}
			}
		})
	}
}
