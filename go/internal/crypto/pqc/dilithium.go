package pqc

import (
	"fmt"

	"github.com/cloudflare/circl/sign"
	"github.com/cloudflare/circl/sign/dilithium/mode3"
)

// DilithiumKeyPair represents a Dilithium-3 key pair
type DilithiumKeyPair struct {
	PublicKey  sign.PublicKey
	PrivateKey sign.PrivateKey
	Scheme     sign.Scheme
}

// GenerateDilithiumKeyPair generates a new Dilithium-3 key pair
func GenerateDilithiumKeyPair() (*DilithiumKeyPair, error) {
	scheme := mode3.Scheme()
	publicKey, privateKey, err := scheme.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate Dilithium key pair: %w", err)
	}

	return &DilithiumKeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		Scheme:     scheme,
	}, nil
}

// DilithiumSign signs a message using Dilithium-3
func DilithiumSign(privateKey sign.PrivateKey, message []byte) ([]byte, error) {
	scheme := mode3.Scheme()
	signature := scheme.Sign(privateKey, message, nil)
	return signature, nil
}

// DilithiumVerify verifies a signature using Dilithium-3
func DilithiumVerify(publicKey sign.PublicKey, message []byte, signature []byte) bool {
	scheme := mode3.Scheme()
	return scheme.Verify(publicKey, message, signature, nil)
}

// MarshalPublicKey converts a public key to bytes
func (kp *DilithiumKeyPair) MarshalPublicKey() ([]byte, error) {
	return kp.PublicKey.MarshalBinary()
}

// MarshalPrivateKey converts a private key to bytes
func (kp *DilithiumKeyPair) MarshalPrivateKey() ([]byte, error) {
	return kp.PrivateKey.MarshalBinary()
}

// UnmarshalPublicKey converts bytes to a public key
func UnmarshalDilithiumPublicKey(data []byte) (sign.PublicKey, error) {
	scheme := mode3.Scheme()
	return scheme.UnmarshalBinaryPublicKey(data)
}

// UnmarshalPrivateKey converts bytes to a private key
func UnmarshalDilithiumPrivateKey(data []byte) (sign.PrivateKey, error) {
	scheme := mode3.Scheme()
	return scheme.UnmarshalBinaryPrivateKey(data)
}
