package security

import (
	"bytes"
	"testing"
)

func TestNewMemoryEncryption(t *testing.T) {
	enc := NewMemoryEncryption()
	if enc == nil {
		t.Fatal("Expected MemoryEncryption, got nil")
	}
	if enc.iterations != 100000 {
		t.Errorf("Expected iterations 100000, got %d", enc.iterations)
	}
	if enc.keyLength != 32 {
		t.Errorf("Expected keyLength 32, got %d", enc.keyLength)
	}
}

func TestDeriveKey(t *testing.T) {
	enc := NewMemoryEncryption()
	salt := []byte("test-salt-1234567890123456") // 16 bytes

	key := enc.DeriveKey("test-secret", salt)
	if len(key) != 32 {
		t.Errorf("Expected key length 32, got %d", len(key))
	}

	// Test that same inputs produce same key
	key2 := enc.DeriveKey("test-secret", salt)
	if !bytes.Equal(key, key2) {
		t.Error("Expected same key for same inputs")
	}

	// Test that different inputs produce different keys
	key3 := enc.DeriveKey("different-secret", salt)
	if bytes.Equal(key, key3) {
		t.Error("Expected different key for different secret")
	}
}

func TestEncryptDecryptMemory(t *testing.T) {
	enc := NewMemoryEncryption()
	key := []byte("12345678901234567890123456789012") // 32 bytes
	plaintext := []byte("This is a test message for encryption")

	// Encrypt
	ciphertext, err := enc.EncryptMemory(plaintext, key)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}
	if len(ciphertext) == 0 {
		t.Error("Expected non-empty ciphertext")
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("Expected ciphertext to be different from plaintext")
	}

	// Decrypt
	decrypted, err := enc.DecryptMemory(ciphertext, key)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Expected decrypted text to match original, got %s", string(decrypted))
	}
}

func TestDecryptMemoryInvalidCiphertext(t *testing.T) {
	enc := NewMemoryEncryption()
	key := []byte("12345678901234567890123456789012")

	// Test with too short ciphertext
	_, err := enc.DecryptMemory([]byte("short"), key)
	if err == nil {
		t.Error("Expected error for too short ciphertext")
	}

	// Test with invalid ciphertext
	_, err = enc.DecryptMemory([]byte("invalid-ciphertext-that-is-long-enough"), key)
	if err == nil {
		t.Error("Expected error for invalid ciphertext")
	}
}

func TestGenerateSalt(t *testing.T) {
	enc := NewMemoryEncryption()

	salt1, err := enc.GenerateSalt()
	if err != nil {
		t.Fatalf("Failed to generate salt: %v", err)
	}
	if len(salt1) != 16 {
		t.Errorf("Expected salt length 16, got %d", len(salt1))
	}

	// Test that salts are random
	salt2, err := enc.GenerateSalt()
	if err != nil {
		t.Fatalf("Failed to generate second salt: %v", err)
	}
	if bytes.Equal(salt1, salt2) {
		t.Error("Expected different salts on multiple calls")
	}
}

func TestEncodeDecodeKey(t *testing.T) {
	enc := NewMemoryEncryption()
	key := []byte("12345678901234567890123456789012")

	// Encode
	encoded := enc.EncodeKey(key)
	if encoded == "" {
		t.Error("Expected non-empty encoded key")
	}

	// Decode
	decoded, err := enc.DecodeKey(encoded)
	if err != nil {
		t.Fatalf("Failed to decode key: %v", err)
	}
	if !bytes.Equal(decoded, key) {
		t.Error("Expected decoded key to match original")
	}
}

func TestDecodeKeyInvalid(t *testing.T) {
	enc := NewMemoryEncryption()

	_, err := enc.DecodeKey("invalid-base64!")
	if err == nil {
		t.Error("Expected error for invalid base64")
	}
}

func TestEncryptMemoryInvalidKey(t *testing.T) {
	enc := NewMemoryEncryption()

	// Test with invalid key length
	invalidKey := []byte("short-key")
	data := []byte("test data")

	_, err := enc.EncryptMemory(data, invalidKey)
	if err == nil {
		t.Error("Expected error for invalid key length")
	}
}

func TestDecryptMemoryInvalidKey(t *testing.T) {
	enc := NewMemoryEncryption()

	// Test with invalid key length
	invalidKey := []byte("short-key")
	ciphertext := []byte("some-ciphertext")

	_, err := enc.DecryptMemory(ciphertext, invalidKey)
	if err == nil {
		t.Error("Expected error for invalid key length")
	}
}
