package pqc_test

import (
	"testing"

	"github.com/knirv/knirvbase/internal/crypto/pqc"
)

func TestKyberEncryptDecrypt(t *testing.T) {
	// Generate key pair
	keyPair, err := pqc.GenerateKyberKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Kyber key pair: %v", err)
	}

	plaintext := []byte("Hello, quantum-resistant world!")

	// Test direct Kyber KEM
	scheme := keyPair.Scheme
	ciphertextKem, sharedSecretEnc, err := scheme.Encapsulate(keyPair.PublicKey)
	if err != nil {
		t.Fatalf("Failed to encapsulate: %v", err)
	}

	t.Logf("Shared secret enc length: %d", len(sharedSecretEnc))
	t.Logf("KEM ciphertext length: %d", len(ciphertextKem))

	sharedSecretDec, err := scheme.Decapsulate(keyPair.PrivateKey, ciphertextKem)
	if err != nil {
		t.Fatalf("Failed to decapsulate: %v", err)
	}

	t.Logf("Shared secret dec length: %d", len(sharedSecretDec))

	if string(sharedSecretEnc) != string(sharedSecretDec) {
		t.Errorf("Shared secrets don't match!")
	}

	// Encrypt
	ciphertext, err := pqc.KyberEncrypt(keyPair.PublicKey, plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	t.Logf("Full ciphertext length: %d", len(ciphertext))

	// Decrypt using the same key pair
	decrypted, err := pqc.KyberDecrypt(keyPair.PrivateKey, ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match original: got %s, want %s", decrypted, plaintext)
	}
}

func TestDilithiumSignVerify(t *testing.T) {
	// Generate key pair
	keyPair, err := pqc.GenerateDilithiumKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Dilithium key pair: %v", err)
	}

	message := []byte("This message will be signed")

	// Sign
	signature, err := pqc.DilithiumSign(keyPair.PrivateKey, message)
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	// Verify
	valid := pqc.DilithiumVerify(keyPair.PublicKey, message, signature)
	if !valid {
		t.Error("Signature verification failed")
	}

	// Test with wrong message
	wrongMessage := []byte("Wrong message")
	valid = pqc.DilithiumVerify(keyPair.PublicKey, wrongMessage, signature)
	if valid {
		t.Error("Signature verification should have failed for wrong message")
	}
}

func TestPQCKeyPair(t *testing.T) {
	// Generate PQC key pair
	keyPair, err := pqc.GeneratePQCKeyPair("test-key", "encryption")
	if err != nil {
		t.Fatalf("Failed to generate PQC key pair: %v", err)
	}

	if keyPair.Name != "test-key" {
		t.Errorf("Expected name 'test-key', got %s", keyPair.Name)
	}

	if keyPair.Purpose != "encryption" {
		t.Errorf("Expected purpose 'encryption', got %s", keyPair.Purpose)
	}

	if !keyPair.IsActive() {
		t.Error("Key pair should be active")
	}

	// Test encryption/decryption
	plaintext := []byte("Secret data")
	ciphertext, err := keyPair.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	decrypted, err := keyPair.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted data doesn't match: got %s, want %s", decrypted, plaintext)
	}

	// Test signing/verification
	message := []byte("Message to sign")
	signature, err := keyPair.Sign(message)
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	valid := keyPair.Verify(message, signature)
	if !valid {
		t.Error("Signature verification failed")
	}
}

func TestEncryptionManager(t *testing.T) {
	em := pqc.NewEncryptionManager()

	// Generate master key
	masterKey, err := pqc.GeneratePQCKeyPair("master", "encryption")
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	em.SetMasterKey(masterKey)
	em.CacheKey(masterKey.ID, masterKey) // Cache the key

	// Test data encryption
	plaintext := []byte("Sensitive data")
	encrypted, err := em.EncryptData(plaintext, masterKey.ID)
	if err != nil {
		t.Fatalf("Failed to encrypt data: %v", err)
	}

	// Test data decryption
	decrypted, err := em.DecryptData(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt data: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted data doesn't match: got %s, want %s", decrypted, plaintext)
	}
}

func BenchmarkKyberEncrypt(b *testing.B) {
	keyPair, _ := pqc.GenerateKyberKeyPair()
	plaintext := []byte("Benchmark test data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pqc.KyberEncrypt(keyPair.PublicKey, plaintext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkKyberDecrypt(b *testing.B) {
	keyPair, _ := pqc.GenerateKyberKeyPair()
	plaintext := []byte("Benchmark test data")
	ciphertext, _ := pqc.KyberEncrypt(keyPair.PublicKey, plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pqc.KyberDecrypt(keyPair.PrivateKey, ciphertext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDilithiumSign(b *testing.B) {
	keyPair, _ := pqc.GenerateDilithiumKeyPair()
	message := []byte("Benchmark test message")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pqc.DilithiumSign(keyPair.PrivateKey, message)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDilithiumVerify(b *testing.B) {
	keyPair, _ := pqc.GenerateDilithiumKeyPair()
	message := []byte("Benchmark test message")
	signature, _ := pqc.DilithiumSign(keyPair.PrivateKey, message)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid := pqc.DilithiumVerify(keyPair.PublicKey, message, signature)
		if !valid {
			b.Fatal("Verification failed")
		}
	}
}
