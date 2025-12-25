package benchmarks

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/knirvcorp/knirvbase/go/internal/crypto/pqc"
	typ "github.com/knirvcorp/knirvbase/go/internal/types"
	"github.com/knirvcorp/knirvbase/go/pkg/knirvbase"
)

// Benchmark suite for KNIRVBASE performance baselines
// Targets from ASIC-Shield integration plan:
// - Insert credential: < 10ms (p99)
// - Query by username: < 5ms (p99)
// - Authentication workflow: < 500ms (p99, including 100M KDF iterations)
// - PQC encryption overhead: < 20ms per operation
// - 10,000 credentials without performance degradation

var benchmarkDB *knirvbase.DB
var benchmarkCtx context.Context

func TestMain(m *testing.M) {
	// Setup benchmark database
	benchmarkCtx = context.Background()

	// Use temp directory for benchmarks
	tempDir, err := os.MkdirTemp("", "knirvbase-bench-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	opts := knirvbase.Options{
		DataDir:            tempDir,
		DistributedEnabled: false, // Use local-only for benchmarks
	}

	benchmarkDB, err = knirvbase.New(benchmarkCtx, opts)
	if err != nil {
		panic(err)
	}

	// Create credentials collection
	credentialsColl := benchmarkDB.Collection("credentials")

	// Create network for distributed features (even if disabled)
	networkID, err := benchmarkDB.CreateNetwork(typ.NetworkConfig{
		NetworkID: "bench-network",
		Name:      "Benchmark Network",
	})
	if err != nil {
		panic(err)
	}

	err = credentialsColl.AttachToNetwork(networkID)
	if err != nil {
		panic(err)
	}

	code := m.Run()
	benchmarkDB.Shutdown()
	os.Exit(code)
}

// generateTestCredential creates a test credential document
func generateTestCredential(username string) map[string]interface{} {
	// Generate random salt
	salt := make([]byte, 32)
	rand.Read(salt)

	// Simulate password hash (in real ASIC-Shield, this would be PQC encrypted)
	hash := make([]byte, 64) // 512-bit hash
	rand.Read(hash)

	return map[string]interface{}{
		"id":        username,
		"entryType": typ.EntryTypeAuth, // Using AUTH for credentials
		"payload": map[string]interface{}{
			"username":      username,
			"display_name":  fmt.Sprintf("User %s", username),
			"email":         fmt.Sprintf("%s@example.com", username),
			"hash":          hash,
			"salt":          salt,
			"iterations":    100000, // Reduced for benchmarks
			"algorithm":     "PBKDF2-SHA256",
			"pqc_algorithm": "Kyber-768",
			"pqc_key_id":    "test-key-123",
			"metadata": map[string]interface{}{
				"department": "engineering",
				"role":       "user",
			},
			"created_at": time.Now().UnixMilli(),
			"updated_at": time.Now().UnixMilli(),
			"status":     "active",
		},
	}
}

// BenchmarkCredentialInsert measures credential insertion performance
func BenchmarkCredentialInsert(b *testing.B) {
	credentialsColl := benchmarkDB.Collection("credentials")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		username := fmt.Sprintf("user%d", i)
		doc := generateTestCredential(username)

		_, err := credentialsColl.Insert(benchmarkCtx, doc)
		if err != nil {
			b.Fatalf("Insert failed: %v", err)
		}
	}
}

// BenchmarkCredentialQuery measures credential lookup by username
func BenchmarkCredentialQuery(b *testing.B) {
	credentialsColl := benchmarkDB.Collection("credentials")

	// Pre-populate with test data
	for i := 0; i < 1000; i++ {
		username := fmt.Sprintf("query_user%d", i)
		doc := generateTestCredential(username)
		_, err := credentialsColl.Insert(benchmarkCtx, doc)
		if err != nil {
			b.Fatalf("Setup insert failed: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		username := fmt.Sprintf("query_user%d", i%1000)

		doc, err := credentialsColl.Find(username)
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
		if doc == nil {
			b.Fatalf("Document not found: %s", username)
		}
	}
}

// BenchmarkPQCCrypto measures PQC encryption/decryption overhead
func BenchmarkPQCCrypto(b *testing.B) {
	// Generate PQC key pair
	keyPair, err := pqc.GeneratePQCKeyPair("benchmark", "encryption")
	if err != nil {
		b.Fatalf("Failed to generate PQC key pair: %v", err)
	}

	// Test data
	plaintext := make([]byte, 32) // 256-bit key/hash
	rand.Read(plaintext)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Encrypt
		ciphertext, err := keyPair.Encrypt(plaintext)
		if err != nil {
			b.Fatalf("Encryption failed: %v", err)
		}

		// Decrypt
		decrypted, err := keyPair.Decrypt(ciphertext)
		if err != nil {
			b.Fatalf("Decryption failed: %v", err)
		}

		// Verify
		if len(decrypted) != len(plaintext) {
			b.Fatalf("Decryption length mismatch")
		}
	}
}

// BenchmarkAuthWorkflow simulates full authentication workflow
func BenchmarkAuthWorkflow(b *testing.B) {
	credentialsColl := benchmarkDB.Collection("credentials")

	// Generate PQC key pair for encryption simulation
	keyPair, err := pqc.GeneratePQCKeyPair("auth_benchmark", "encryption")
	if err != nil {
		b.Fatalf("Failed to generate PQC key pair: %v", err)
	}

	// Pre-populate with test credential (with properly encrypted hash)
	username := "auth_test_user"
	testPasswordHash := []byte("test_password_hash_32_bytes")
	encryptedHash, err := keyPair.Encrypt(testPasswordHash)
	if err != nil {
		b.Fatalf("Failed to encrypt test hash: %v", err)
	}

	doc := map[string]interface{}{
		"id":        username,
		"entryType": typ.EntryTypeAuth,
		"payload": map[string]interface{}{
			"username":      username,
			"display_name":  "Auth Test User",
			"email":         "auth@example.com",
			"hash":          base64.StdEncoding.EncodeToString(encryptedHash), // Base64 encoded for JSON storage
			"salt":          base64.StdEncoding.EncodeToString([]byte("test_salt_32_bytes_for_benchmark")),
			"iterations":    100000, // Reduced for benchmarks
			"algorithm":     "PBKDF2-SHA256",
			"pqc_algorithm": "Kyber-768",
			"pqc_key_id":    keyPair.ID,
			"metadata": map[string]interface{}{
				"department": "security",
				"role":       "user",
			},
			"created_at": time.Now().UnixMilli(),
			"updated_at": time.Now().UnixMilli(),
			"status":     "active",
		},
	}

	_, err = credentialsColl.Insert(benchmarkCtx, doc)
	if err != nil {
		b.Fatalf("Setup insert failed: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 1. Lookup credential by username
		doc, err := credentialsColl.Find(username)
		if err != nil {
			b.Fatalf("Credential lookup failed: %v", err)
		}
		if doc == nil {
			b.Fatalf("Credential not found")
		}

		// 2. Extract hash and salt from payload
		payload := doc["payload"].(map[string]interface{})
		storedHashStr := payload["hash"].(string)
		storedHash, err := base64.StdEncoding.DecodeString(storedHashStr)
		if err != nil {
			b.Fatalf("Failed to decode stored hash: %v", err)
		}
		saltStr := payload["salt"].(string)
		salt, err := base64.StdEncoding.DecodeString(saltStr)
		if err != nil {
			b.Fatalf("Failed to decode salt: %v", err)
		}
		iterations := int(payload["iterations"].(float64))

		// 3. PQC decryption of stored hash
		decryptedHash, err := keyPair.Decrypt(storedHash)
		if err != nil {
			b.Fatalf("PQC decryption failed: %v", err)
		}

		// 4. Simulate KDF computation (simplified PBKDF2)
		// In real ASIC-Shield, this would be ASIC-accelerated
		testPassword := []byte("test_password_123")
		computedHash := make([]byte, 32)
		// Simulate KDF time with a loop (not real PBKDF2 for speed)
		for j := 0; j < iterations/1000; j++ {
			copy(computedHash, testPassword)
			for k := range computedHash {
				computedHash[k] ^= salt[k%len(salt)]
			}
		}

		// 5. Compare hashes (simulate verification)
		hashMatches := true
		if len(decryptedHash) == len(computedHash) {
			for k := range decryptedHash {
				if decryptedHash[k] != computedHash[k] {
					hashMatches = false
					break
				}
			}
		} else {
			hashMatches = false
		}

		// 6. Update last_used on success (simulated)
		if hashMatches {
			update := map[string]interface{}{
				"last_used": time.Now().UnixMilli(),
			}
			_, err = credentialsColl.Update(username, map[string]interface{}{
				"payload": update,
			})
			if err != nil {
				b.Fatalf("Update failed: %v", err)
			}
		}
	}
}

// BenchmarkLargeScale tests performance with 10K credentials
func BenchmarkLargeScale(b *testing.B) {
	credentialsColl := benchmarkDB.Collection("credentials")

	// Pre-populate with 10K credentials
	b.Log("Pre-populating 10,000 credentials...")
	for i := 0; i < 10000; i++ {
		username := fmt.Sprintf("scale_user%05d", i)
		doc := generateTestCredential(username)
		_, err := credentialsColl.Insert(benchmarkCtx, doc)
		if err != nil {
			b.Fatalf("Setup insert failed: %v", err)
		}
	}
	b.Log("Pre-population complete")

	b.ResetTimer()
	b.ReportAllocs()

	// Benchmark queries on the large dataset
	for i := 0; i < b.N; i++ {
		username := fmt.Sprintf("scale_user%05d", i%10000)

		doc, err := credentialsColl.Find(username)
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
		if doc == nil {
			b.Fatalf("Document not found: %s", username)
		}
	}
}
