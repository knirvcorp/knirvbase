package pqc

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudflare/circl/kem"
	"github.com/cloudflare/circl/sign"
)

// PQCKeyPair represents a complete PQC key pair with both Kyber and Dilithium keys
type PQCKeyPair struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Purpose   string     `json:"purpose"` // encryption, signature, kex
	Algorithm string     `json:"algorithm"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Status    string     `json:"status"` // active, rotated, revoked, expired

	// Kyber keys for encryption
	KyberPublicKey  kem.PublicKey  `json:"-"`
	KyberPrivateKey kem.PrivateKey `json:"-"`

	// Dilithium keys for signatures
	DilithiumPublicKey  sign.PublicKey  `json:"-"`
	DilithiumPrivateKey sign.PrivateKey `json:"-"`

	// Marshaled versions for storage
	KyberPublicKeyBytes      []byte `json:"kyber_public_key"`
	KyberPrivateKeyBytes     []byte `json:"kyber_private_key,omitempty"` // encrypted in storage
	DilithiumPublicKeyBytes  []byte `json:"dilithium_public_key,omitempty"`
	DilithiumPrivateKeyBytes []byte `json:"dilithium_private_key,omitempty"` // encrypted in storage
}

// GeneratePQCKeyPair generates a new PQC key pair with both Kyber and Dilithium keys
func GeneratePQCKeyPair(name, purpose string) (*PQCKeyPair, error) {
	// Generate Kyber key pair
	kyberPair, err := GenerateKyberKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate Kyber keys: %w", err)
	}

	// Generate Dilithium key pair
	dilithiumPair, err := GenerateDilithiumKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate Dilithium keys: %w", err)
	}

	// Generate unique ID
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("failed to generate ID: %w", err)
	}
	id := fmt.Sprintf("%x", idBytes)

	// Marshal keys to bytes
	kyberPubBytes, err := kyberPair.MarshalPublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Kyber public key: %w", err)
	}

	kyberPrivBytes, err := kyberPair.MarshalPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Kyber private key: %w", err)
	}

	dilithiumPubBytes, err := dilithiumPair.MarshalPublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Dilithium public key: %w", err)
	}

	dilithiumPrivBytes, err := dilithiumPair.MarshalPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Dilithium private key: %w", err)
	}

	return &PQCKeyPair{
		ID:                       id,
		Name:                     name,
		Purpose:                  purpose,
		Algorithm:                "Kyber-768+Dilithium-3",
		CreatedAt:                time.Now(),
		Status:                   "active",
		KyberPublicKey:           kyberPair.PublicKey,
		KyberPrivateKey:          kyberPair.PrivateKey,
		DilithiumPublicKey:       dilithiumPair.PublicKey,
		DilithiumPrivateKey:      dilithiumPair.PrivateKey,
		KyberPublicKeyBytes:      kyberPubBytes,
		KyberPrivateKeyBytes:     kyberPrivBytes,
		DilithiumPublicKeyBytes:  dilithiumPubBytes,
		DilithiumPrivateKeyBytes: dilithiumPrivBytes,
	}, nil
}

// LoadPQCKeyPair loads a PQC key pair from marshaled data
func LoadPQCKeyPair(data []byte) (*PQCKeyPair, error) {
	var kp PQCKeyPair
	if err := json.Unmarshal(data, &kp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key pair: %w", err)
	}

	// Unmarshal Kyber keys
	if len(kp.KyberPublicKeyBytes) > 0 {
		pubKey, err := UnmarshalKyberPublicKey(kp.KyberPublicKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal Kyber public key: %w", err)
		}
		kp.KyberPublicKey = pubKey
	}

	if len(kp.KyberPrivateKeyBytes) > 0 {
		privKey, err := UnmarshalKyberPrivateKey(kp.KyberPrivateKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal Kyber private key: %w", err)
		}
		kp.KyberPrivateKey = privKey
	}

	// Unmarshal Dilithium keys
	if len(kp.DilithiumPublicKeyBytes) > 0 {
		pubKey, err := UnmarshalDilithiumPublicKey(kp.DilithiumPublicKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal Dilithium public key: %w", err)
		}
		kp.DilithiumPublicKey = pubKey
	}

	if len(kp.DilithiumPrivateKeyBytes) > 0 {
		privKey, err := UnmarshalDilithiumPrivateKey(kp.DilithiumPrivateKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal Dilithium private key: %w", err)
		}
		kp.DilithiumPrivateKey = privKey
	}

	return &kp, nil
}

// Marshal serializes the key pair to JSON (without private keys for public storage)
func (kp *PQCKeyPair) Marshal() ([]byte, error) {
	// Create a copy without private key bytes for public marshaling
	publicKp := *kp
	publicKp.KyberPrivateKeyBytes = nil
	publicKp.DilithiumPrivateKeyBytes = nil

	return json.Marshal(publicKp)
}

// MarshalWithPrivateKeys serializes the key pair to JSON including private keys
// WARNING: This should only be used for encrypted storage
func (kp *PQCKeyPair) MarshalWithPrivateKeys() ([]byte, error) {
	return json.Marshal(kp)
}

// Encrypt encrypts data using the Kyber public key
func (kp *PQCKeyPair) Encrypt(plaintext []byte) ([]byte, error) {
	if kp.KyberPublicKey == nil {
		return nil, fmt.Errorf("no Kyber public key available")
	}
	return KyberEncrypt(kp.KyberPublicKey, plaintext)
}

// Decrypt decrypts data using the Kyber private key
func (kp *PQCKeyPair) Decrypt(ciphertext []byte) ([]byte, error) {
	if kp.KyberPrivateKey == nil {
		return nil, fmt.Errorf("no Kyber private key available")
	}
	return KyberDecrypt(kp.KyberPrivateKey, ciphertext)
}

// Sign signs data using the Dilithium private key
func (kp *PQCKeyPair) Sign(message []byte) ([]byte, error) {
	if kp.DilithiumPrivateKey == nil {
		return nil, fmt.Errorf("no Dilithium private key available")
	}
	return DilithiumSign(kp.DilithiumPrivateKey, message)
}

// Verify verifies a signature using the Dilithium public key
func (kp *PQCKeyPair) Verify(message []byte, signature []byte) bool {
	if kp.DilithiumPublicKey == nil {
		return false
	}
	return DilithiumVerify(kp.DilithiumPublicKey, message, signature)
}

// IsExpired checks if the key pair has expired
func (kp *PQCKeyPair) IsExpired() bool {
	if kp.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*kp.ExpiresAt)
}

// IsActive checks if the key pair is active and not expired
func (kp *PQCKeyPair) IsActive() bool {
	return kp.Status == "active" && !kp.IsExpired()
}
