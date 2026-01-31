package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

type MemoryEncryption struct {
	iterations int
	keyLength  int
}

func NewMemoryEncryption() *MemoryEncryption {
	return &MemoryEncryption{
		iterations: 100000,
		keyLength:  32,
	}
}

// DeriveKey derives an encryption key from user secret
func (m *MemoryEncryption) DeriveKey(userSecret string, salt []byte) []byte {
	return pbkdf2.Key(
		[]byte(userSecret),
		salt,
		m.iterations,
		m.keyLength,
		sha256.New,
	)
}

// EncryptMemory encrypts memory data before storage
func (m *MemoryEncryption) EncryptMemory(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// DecryptMemory decrypts memory data for retrieval
func (m *MemoryEncryption) DecryptMemory(encrypted []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// GenerateSalt generates a random salt for key derivation
func (m *MemoryEncryption) GenerateSalt() ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// EncodeKey encodes a key to base64 for storage
func (m *MemoryEncryption) EncodeKey(key []byte) string {
	return base64.URLEncoding.EncodeToString(key)
}

// DecodeKey decodes a base64-encoded key
func (m *MemoryEncryption) DecodeKey(encoded string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(encoded)
}